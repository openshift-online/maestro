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
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	workv1 "open-cluster-management.io/api/work/v1"
)

var _ = Describe("Certificate Rotation", Ordered, Label("e2e-tests-cert-rotation"), func() {
	Context("Certificate Rotation Tests", func() {
		var workName string
		var deployName string
		var originalMQTTCerts map[string][]byte
		var originalGRPCBrokerCerts map[string][]byte

		BeforeAll(func() {
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
		})

		AfterAll(func() {
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

			// wait for certificate reload and agent reconnection
			time.Sleep(30 * time.Second)

			// clean up the test resource
			By(fmt.Sprintf("cleaning up test work %s", workName))
			_ = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
			Eventually(func() error {
				return AssertWorkNotFound(workName)
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())

			// ensure the deployment is deleted from agent
			Eventually(func() error {
				_, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("nginx deployment still exists")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		It("should verify agent works with current certificates", func() {
			By("creating a test work with deployment (1 replica) to verify agent connectivity")
			work := helper.NewManifestWork(workName, deployName, "default", 1)
			_, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Create(ctx, work, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("verifying the deployment is applied on the agent cluster")
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

		It("should rotate client certificate with short expiration for maestro agent", func() {
			rotated, err := rotateCertificates(ctx, 60*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(rotated).To(BeTrue(), "no CA secrets found; certificate rotation did not run")

			By("waiting for certificate refresh...")
			time.Sleep(10 * time.Second)
		})

		It("should verify agent can process updates with new certificate", func() {
			By("checking agent pod is still running")
			pods, err := agentTestOpts.kubeClientSet.CoreV1().Pods(agentTestOpts.agentNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=maestro-agent",
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(pods.Items)).Should(BeNumerically(">", 0))

			// check pod is in Running state
			pod := pods.Items[0]
			Expect(pod.Status.Phase).Should(Equal(corev1.PodRunning))

			By("updating the deployment to 2 replicas to trigger agent communication")
			work, err := sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Get(ctx, workName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			newWork := work.DeepCopy()
			newWork.Spec.Workload.Manifests = []workv1.Manifest{helper.NewManifest(deployName, "default", 2)}
			patchData, err := grpcsource.ToWorkPatch(work, newWork)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = sourceWorkClient.ManifestWorks(agentTestOpts.consumerName).Patch(ctx, workName, types.MergePatchType, patchData, metav1.PatchOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			By("verifying the deployment is updated to 2 replicas before agent certificate expiration...")
			Eventually(func() error {
				deployment, err := agentTestOpts.kubeClientSet.AppsV1().Deployments("default").Get(ctx, deployName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas != 2 {
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
