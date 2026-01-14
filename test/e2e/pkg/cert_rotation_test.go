package e2e_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
)

var _ = Describe("Certificate Rotation", Ordered, Label("e2e-tests-cert-rotation"), func() {
	Context("Certificate Rotation Tests", func() {
		var workName string
		var deployName string
		var originalMQTTCerts map[string][]byte
		var originalGRPCBrokerCerts map[string][]byte
		var skip bool

		BeforeAll(func() {
			// Check if any CA secrets exist (MQTT or gRPC)
			// Certificate rotation is only applicable for MQTT and gRPC brokers
			// Pub/Sub emulator does not use client certificates
			_, mqttErr := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-mqtt-ca", metav1.GetOptions{})
			_, grpcErr := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-grpc-broker-ca", metav1.GetOptions{})

			if errors.IsNotFound(mqttErr) && errors.IsNotFound(grpcErr) {
				skip = true
				Skip("Skipping certificate rotation tests: no CA secrets found")
			}

			if mqttErr != nil && !errors.IsNotFound(mqttErr) {
				Expect(mqttErr).ShouldNot(HaveOccurred())
			}
			if grpcErr != nil && !errors.IsNotFound(grpcErr) {
				Expect(grpcErr).ShouldNot(HaveOccurred())
			}

			workName = fmt.Sprintf("cert-rotation-%s", k8srand.String(5))
			deployName = fmt.Sprintf("nginx-%s", k8srand.String(5))

			By("saving original MQTT certificate")
			secret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-agent-certs", metav1.GetOptions{})
			if err == nil {
				originalMQTTCerts = make(map[string][]byte)
				for key, value := range secret.Data {
					originalMQTTCerts[key] = make([]byte, len(value))
					copy(originalMQTTCerts[key], value)
				}
			} else if !errors.IsNotFound(err) {
				Expect(err).ShouldNot(HaveOccurred())
			}

			By("saving original gRPC broker certificate")
			grpcSecret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-grpc-broker-cert", metav1.GetOptions{})
			if err == nil {
				originalGRPCBrokerCerts = make(map[string][]byte)
				for key, value := range grpcSecret.Data {
					originalGRPCBrokerCerts[key] = make([]byte, len(value))
					copy(originalGRPCBrokerCerts[key], value)
				}
			} else if !errors.IsNotFound(err) {
				Expect(err).ShouldNot(HaveOccurred())
			}

			By("rotating certificate with 30 seconds expiration")
			rotated, err := rotateCertificates(ctx, 30*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(rotated).To(BeTrue(), "no CA secrets found; certificate rotation did not run")

			By("restarting maestro-agent to quickly pick up new certificate")
			err = restartDeployment(ctx, agentTestOpts.kubeClientSet, "maestro-agent", agentTestOpts.agentNamespace)
			Expect(err).ShouldNot(HaveOccurred())

			By("creating a test work with deployment (1 replica)")
			work := helper.NewManifestWork(workName, deployName, "default", 1)
			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("verifying the deployment is created on the agent cluster")
			Eventually(func() error {
				deployment, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas != 1 {
					return fmt.Errorf("expected 1 replica, got %d", *deployment.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			By("verifying work status is reported back")
			Eventually(func() error {
				work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if !meta.IsStatusConditionTrue(work.Status.Conditions, "Applied") {
					return fmt.Errorf("work not applied yet")
				}
				if !meta.IsStatusConditionTrue(work.Status.Conditions, "Available") {
					return fmt.Errorf("work not available yet")
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			if skip {
				return
			}

			By("restoring original MQTT certificate")
			if len(originalMQTTCerts) > 0 {
				secret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-agent-certs", metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				secret.Data = originalMQTTCerts
				_, err = agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Update(ctx, secret, metav1.UpdateOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			}

			By("restoring original gRPC broker certificate")
			if len(originalGRPCBrokerCerts) > 0 {
				secret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-grpc-broker-cert", metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				secret.Data = originalGRPCBrokerCerts
				_, err = agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Update(ctx, secret, metav1.UpdateOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			}

			By("restarting maestro-agent to quickly pick up restored certificate")
			err := restartDeployment(ctx, agentTestOpts.kubeClientSet, "maestro-agent", agentTestOpts.agentNamespace)
			Expect(err).ShouldNot(HaveOccurred())

			By(fmt.Sprintf("deleting test work %s", workName))
			err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("ensuring the work is deleted")
			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			By("ensuring the deployment is deleted from agent cluster")
			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("deployment %s still exists", deployName)
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("should update work when certificate expires and succeed after rotation", func() {
			By("waiting for the certificate to expire (30 seconds)")
			time.Sleep(30 * time.Second)

			By("rotating certificate with long expiration (1 hour)")
			rotated, err := rotateCertificates(ctx, 1*time.Hour)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(rotated).To(BeTrue(), "certificate rotation did not run")

			By("waiting 10 seconds for certificate reload")
			time.Sleep(10 * time.Second)

			By("updating the work to change replicas to 2")
			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{helper.NewManifest(deployName, "default", 2)}
			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("eventually verifying the deployment replicas is updated to 2 (certificate refreshed)")
			Eventually(func() error {
				deployment, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if deployment.Spec.Replicas == nil {
					return fmt.Errorf("deployment replicas is nil")
				}
				if *deployment.Spec.Replicas != 2 {
					return fmt.Errorf("expected 2 replicas, got %d", *deployment.Spec.Replicas)
				}
				return nil
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})
})

// rotateCertificates rotates both MQTT and gRPC broker client certificates with the specified duration
func rotateCertificates(ctx context.Context, duration time.Duration) (bool, error) {
	rotated := false
	if duration <= 0 {
		return false, fmt.Errorf("duration must be > 0, got %s", duration)
	}
	mqttCASecret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-mqtt-ca", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// MQTT rotation not applicable - CA secret not found
	} else if err != nil {
		return false, fmt.Errorf("failed to get maestro-mqtt-ca secret: %w", err)
	} else {
		mqttCACertPEM := mqttCASecret.Data["ca.crt"]
		mqttCAKeyPEM := mqttCASecret.Data["ca.key"]
		if len(mqttCACertPEM) == 0 {
			return false, fmt.Errorf("ca.crt not found in maestro-mqtt-ca secret")
		}
		if len(mqttCAKeyPEM) == 0 {
			return false, fmt.Errorf("ca.key not found in maestro-mqtt-ca secret")
		}

		mqttCACert, err := parseCertificate(mqttCACertPEM)
		if err != nil {
			return false, fmt.Errorf("failed to parse MQTT CA certificate: %w", err)
		}
		mqttCAKey, err := parsePrivateKey(mqttCAKeyPEM)
		if err != nil {
			return false, fmt.Errorf("failed to parse MQTT CA key: %w", err)
		}

		newMQTTClientCertPEM, newMQTTClientKeyPEM, err := signClientCertificate(mqttCACert, mqttCAKey, duration)
		if err != nil {
			return false, fmt.Errorf("failed to sign MQTT client certificate: %w", err)
		}

		mqttSecret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-agent-certs", metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get maestro-agent-certs secret: %w", err)
		}
		if mqttSecret.Data == nil {
			mqttSecret.Data = map[string][]byte{}
		}
		mqttSecret.Data["client.crt"] = newMQTTClientCertPEM
		mqttSecret.Data["client.key"] = newMQTTClientKeyPEM
		_, err = agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Update(ctx, mqttSecret, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update maestro-agent-certs secret: %w", err)
		}
		rotated = true
	}

	gRPCBrokerCASecret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-grpc-broker-ca", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// gRPC rotation not applicable - CA secret not found
	} else if err != nil {
		return false, fmt.Errorf("failed to get maestro-grpc-broker-ca secret: %w", err)
	} else {
		gRPCBrokerCACertPEM := gRPCBrokerCASecret.Data["ca.crt"]
		gRPCBrokerCAKeyPEM := gRPCBrokerCASecret.Data["ca.key"]
		if len(gRPCBrokerCACertPEM) == 0 {
			return false, fmt.Errorf("ca.crt not found in maestro-grpc-broker-ca secret")
		}
		if len(gRPCBrokerCAKeyPEM) == 0 {
			return false, fmt.Errorf("ca.key not found in maestro-grpc-broker-ca secret")
		}

		gRPCBrokerCACert, err := parseCertificate(gRPCBrokerCACertPEM)
		if err != nil {
			return false, fmt.Errorf("failed to parse gRPC broker CA certificate: %w", err)
		}
		gRPCBrokerCAKey, err := parsePrivateKey(gRPCBrokerCAKeyPEM)
		if err != nil {
			return false, fmt.Errorf("failed to parse gRPC broker CA key: %w", err)
		}

		newGRPCClientCertPEM, newGRPCClientKeyPEM, err := signClientCertificate(gRPCBrokerCACert, gRPCBrokerCAKey, duration)
		if err != nil {
			return false, fmt.Errorf("failed to sign gRPC client certificate: %w", err)
		}

		grpcSecret, err := agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Get(ctx, "maestro-grpc-broker-cert", metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get maestro-grpc-broker-cert secret: %w", err)
		}
		if grpcSecret.Data == nil {
			grpcSecret.Data = map[string][]byte{}
		}
		grpcSecret.Data["client.crt"] = newGRPCClientCertPEM
		grpcSecret.Data["client.key"] = newGRPCClientKeyPEM
		_, err = agentTestOpts.kubeClientSet.CoreV1().Secrets(agentTestOpts.agentNamespace).Update(ctx, grpcSecret, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update maestro-grpc-broker-cert secret: %w", err)
		}
		rotated = true
	}

	return rotated, nil
}

// parseCertificate parses a PEM-encoded certificate
func parseCertificate(certPEM []byte) (*x509.Certificate, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}
	return x509.ParseCertificate(certBlock.Bytes)
}

// parsePrivateKey parses a PEM-encoded RSA private key, supports both PKCS1 and PKCS8 formats
func parsePrivateKey(keyPEM []byte) (*rsa.PrivateKey, error) {
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM private key")
	}

	// try PKCS1 format first
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err == nil {
		return key, nil
	}

	// try PKCS8 format
	keyParsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key (tried both PKCS1 and PKCS8): %w", err)
	}

	rsaKey, ok := keyParsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("parsed key is not an RSA private key")
	}

	return rsaKey, nil
}

// signClientCertificate signs a new client certificate and key signed by the provided CA
func signClientCertificate(caCert *x509.Certificate, caKey *rsa.PrivateKey, duration time.Duration) (certPEM, keyPEM []byte, err error) {
	// Generate new private key
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	now := time.Now()

	// create certificate template
	clientCertTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName: "test-client",
		},
		NotBefore:   now.UTC(),
		NotAfter:    now.Add(duration).UTC(),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// create certificate
	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientCertTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// encode to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})

	return certPEM, keyPEM, nil
}

// restartDeployment restarts a deployment by adding the "kubectl.kubernetes.io/restartedAt" annotation
func restartDeployment(ctx context.Context, kubeClient kubernetes.Interface, deploymentName, namespace string) error {
	// Get the deployment
	deployment, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", namespace, deploymentName, err)
	}

	// Add restart annotation to trigger rollout restart
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	// Update the deployment
	_, err = kubeClient.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment %s/%s: %w", namespace, deploymentName, err)
	}

	// Wait for the rollout to complete
	Eventually(func() error {
		deploy, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if deploy.Spec.Replicas == nil {
			return fmt.Errorf("deployment %s/%s has nil .spec.replicas", namespace, deploymentName)
		}
		desired := *deploy.Spec.Replicas

		// Check if the deployment is ready
		if deploy.Status.UpdatedReplicas != desired {
			return fmt.Errorf("waiting for rollout: updated replicas %d/%d", deploy.Status.UpdatedReplicas, desired)
		}
		if deploy.Status.ReadyReplicas != desired {
			return fmt.Errorf("waiting for rollout: ready replicas %d/%d", deploy.Status.ReadyReplicas, desired)
		}
		if deploy.Status.AvailableReplicas != desired {
			return fmt.Errorf("waiting for rollout: available replicas %d/%d", deploy.Status.AvailableReplicas, desired)
		}

		return nil
	}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())

	// Give the deployment a moment to establish connections
	time.Sleep(5 * time.Second)

	return nil
}
