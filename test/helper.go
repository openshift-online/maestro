package test

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/logger"
	mqttoptions "open-cluster-management.io/api/cloudevents/generic/options/mqtt"
	"open-cluster-management.io/api/cloudevents/work"
	"open-cluster-management.io/api/cloudevents/work/agent/codec"

	"github.com/bxcodec/faker/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/segmentio/ksuid"
	"github.com/spf13/pflag"

	amv1 "github.com/openshift-online/ocm-sdk-go/accountsmgmt/v1"

	"github.com/openshift-online/maestro/cmd/maestro/environments"
	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/test/mocks"
)

const (
	apiPort    = ":8777"
	jwtKeyFile = "test/support/jwt_private_key.pem"
	jwtCAFile  = "test/support/jwt_ca.pem"
	jwkKID     = "uhctestkey"
	jwkAlg     = "RS256"
)

var helper *Helper
var once sync.Once

// TODO jwk mock server needs to be refactored out of the helper and into the testing environment
var jwkURL string

// TimeFunc defines a way to get a new Time instance common to the entire test suite.
// Aria's environment has Virtual Time that may not be actual time. We compensate
// by synchronizing on a common time func attached to the test harness.
type TimeFunc func() time.Time

type Helper struct {
	Ctx context.Context

	DBFactory         db.SessionFactory
	AppConfig         *config.ApplicationConfig
	APIServer         server.Server
	MetricsServer     server.Server
	HealthCheckServer server.Server
	ControllerManager *server.ControllersServer
	WorkAgentHolder   *work.ClientHolder
	TimeFunc          TimeFunc
	JWTPrivateKey     *rsa.PrivateKey
	JWTCA             *rsa.PublicKey
	T                 *testing.T
	teardowns         []func() error
}

func NewHelper(t *testing.T) *Helper {
	once.Do(func() {
		jwtKey, jwtCA, err := parseJWTKeys()
		if err != nil {
			fmt.Println("Unable to read JWT keys - this may affect tests that make authenticated server requests")
		}

		env := environments.Environment()
		// Manually set environment name, ignoring environment variables
		env.Name = environments.TestingEnv
		err = env.AddFlags(pflag.CommandLine)
		if err != nil {
			glog.Fatalf("Unable to add environment flags: %s", err.Error())
		}
		if logLevel := os.Getenv("LOGLEVEL"); logLevel != "" {
			glog.Infof("Using custom loglevel: %s", logLevel)
			pflag.CommandLine.Set("-v", logLevel)
		}
		pflag.Parse()

		err = env.Initialize()
		if err != nil {
			glog.Fatalf("Unable to initialize testing environment: %s", err.Error())
		}

		helper = &Helper{
			Ctx:           context.Background(),
			AppConfig:     env.Config,
			DBFactory:     env.Database.SessionFactory,
			JWTPrivateKey: jwtKey,
			JWTCA:         jwtCA,
		}

		// TODO jwk mock server needs to be refactored out of the helper and into the testing environment
		jwkMockTeardown := helper.StartJWKCertServerMock()
		helper.teardowns = []func() error{
			helper.CleanDB,
			jwkMockTeardown,
			helper.stopAPIServer,
		}
		helper.startAPIServer()
		helper.startMetricsServer()
		helper.startHealthCheckServer()
	})
	helper.T = t
	return helper
}

func (helper *Helper) Env() *environments.Env {
	return environments.Environment()
}

func (helper *Helper) Teardown() {
	for _, f := range helper.teardowns {
		err := f()
		if err != nil {
			helper.T.Errorf("error running teardown func: %s", err)
		}
	}
}

func (helper *Helper) startAPIServer() {
	// TODO jwk mock server needs to be refactored out of the helper and into the testing environment
	helper.Env().Config.HTTPServer.JwkCertURL = jwkURL
	helper.APIServer = server.NewAPIServer()
	go func() {
		glog.V(10).Info("Test API server started")
		helper.APIServer.Start()
		glog.V(10).Info("Test API server stopped")
	}()
}

func (helper *Helper) stopAPIServer() error {
	if err := helper.APIServer.Stop(); err != nil {
		return fmt.Errorf("unable to stop api server: %s", err.Error())
	}
	return nil
}

func (helper *Helper) startMetricsServer() {
	helper.MetricsServer = server.NewMetricsServer()
	go func() {
		glog.V(10).Info("Test Metrics server started")
		helper.MetricsServer.Start()
		glog.V(10).Info("Test Metrics server stopped")
	}()
}

func (helper *Helper) stopMetricsServer() {
	if err := helper.MetricsServer.Stop(); err != nil {
		glog.Fatalf("Unable to stop metrics server: %s", err.Error())
	}
}

func (helper *Helper) startHealthCheckServer() {
	helper.HealthCheckServer = server.NewHealthCheckServer()
	go func() {
		glog.V(10).Info("Test health check server started")
		helper.HealthCheckServer.Start()
		glog.V(10).Info("Test health check server stopped")
	}()
}

func (helper *Helper) StartControllerManager(ctx context.Context) {
	sourceClient := helper.Env().Clients.CloudEventsSource
	helper.ControllerManager = &server.ControllersServer{
		KindControllerManager: controllers.NewKindControllerManager(
			// db.NewAdvisoryLockFactory(helper.DBFactory),
			db.NewAdvisoryLockFactory(helper.Env().Database.SessionFactory),
			helper.Env().Services.Events(),
		),
	}

	helper.ControllerManager.KindControllerManager.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {sourceClient.OnCreate},
			api.UpdateEventType: {sourceClient.OnUpdate},
			api.DeleteEventType: {sourceClient.OnDelete},
		},
	})

	go func() {
		glog.V(10).Info("Test controller manager started")
		helper.DBFactory.NewListener(ctx, "events", helper.ControllerManager.KindControllerManager.Handle)
		glog.V(10).Info("Test controller manager stopped")
	}()
}

func (helper *Helper) StartWorkAgent(ctx context.Context, clusterName string, mqttOptions *mqttoptions.MQTTOptions) {
	clientHolder, err := work.NewClientHolderBuilder(clusterName, mqttOptions).
		WithClusterName(clusterName).
		WithCodecs(codec.NewManifestCodec(nil)).
		NewClientHolder(ctx)
	if err != nil {
		glog.Fatalf("Unable to create work agent holder: %s", err)
	}

	go clientHolder.ManifestWorkInformer().Informer().Run(ctx.Done())
	helper.WorkAgentHolder = clientHolder
}

func (helper *Helper) RestartServer() {
	helper.stopAPIServer()
	helper.startAPIServer()
	glog.V(10).Info("Test API server restarted")
}

func (helper *Helper) RestartMetricsServer() {
	helper.stopMetricsServer()
	helper.startMetricsServer()
	glog.V(10).Info("Test metrics server restarted")
}

func (helper *Helper) Reset() {
	glog.Infof("Reseting testing environment")
	env := environments.Environment()
	// Reset the configuration
	env.Config = config.NewApplicationConfig()

	// Re-read command-line configuration into a NEW flagset
	// This new flag set ensures we don't hit conflicts defining the same flag twice
	// Also on reset, we don't care to be re-defining 'v' and other glog flags
	flagset := pflag.NewFlagSet(helper.NewID(), pflag.ContinueOnError)
	env.AddFlags(flagset)
	pflag.Parse()

	err := env.Initialize()
	if err != nil {
		glog.Fatalf("Unable to reset testing environment: %s", err.Error())
	}
	helper.AppConfig = env.Config
	helper.RestartServer()
}

// NewID creates a new unique ID used internally to CS
func (helper *Helper) NewID() string {
	return ksuid.New().String()
}

// NewUUID creates a new unique UUID, which has different formatting than ksuid
// UUID is used by telemeter and we validate the format.
func (helper *Helper) NewUUID() string {
	return uuid.New().String()
}

func (helper *Helper) RestURL(path string) string {
	protocol := "http"
	if helper.AppConfig.HTTPServer.EnableHTTPS {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s/api/maestro/v1%s", protocol, helper.AppConfig.HTTPServer.BindAddress, path)
}

func (helper *Helper) MetricsURL(path string) string {
	return fmt.Sprintf("http://%s%s", helper.AppConfig.Metrics.BindAddress, path)
}

func (helper *Helper) HealthCheckURL(path string) string {
	return fmt.Sprintf("http://%s%s", helper.AppConfig.HealthCheck.BindAddress, path)
}

func (helper *Helper) NewApiClient() *openapi.APIClient {
	config := openapi.NewConfiguration()
	client := openapi.NewAPIClient(config)
	return client
}

func (helper *Helper) NewRandAccount() *amv1.Account {
	return helper.NewAccount(helper.NewID(), faker.Name(), faker.Email())
}

func (helper *Helper) NewAccount(username, name, email string) *amv1.Account {
	var firstName string
	var lastName string
	names := strings.SplitN(name, " ", 2)
	if len(names) < 2 {
		firstName = name
		lastName = ""
	} else {
		firstName = names[0]
		lastName = names[1]
	}

	builder := amv1.NewAccount().
		Username(username).
		FirstName(firstName).
		LastName(lastName).
		Email(email)

	acct, err := builder.Build()
	if err != nil {
		helper.T.Errorf(fmt.Sprintf("Unable to build account: %s", err))
	}
	return acct
}

func (helper *Helper) NewAuthenticatedContext(account *amv1.Account) context.Context {
	tokenString := helper.CreateJWTString(account)
	return context.WithValue(context.Background(), openapi.ContextAccessToken, tokenString)
}

func (helper *Helper) StartJWKCertServerMock() (teardown func() error) {
	jwkURL, teardown = mocks.NewJWKCertServerMock(helper.T, helper.JWTCA, jwkKID, jwkAlg)
	helper.Env().Config.HTTPServer.JwkCertURL = jwkURL
	return teardown
}

func (helper *Helper) DeleteAll(table interface{}) {
	g2 := helper.DBFactory.New(context.Background())
	err := g2.Model(table).Unscoped().Delete(table).Error
	if err != nil {
		helper.T.Errorf("error deleting from table %v: %v", table, err)
	}
}

func (helper *Helper) Delete(obj interface{}) {
	g2 := helper.DBFactory.New(context.Background())
	err := g2.Unscoped().Delete(obj).Error
	if err != nil {
		helper.T.Errorf("error deleting object %v: %v", obj, err)
	}
}

func (helper *Helper) SkipIfShort() {
	if testing.Short() {
		helper.T.Skip("Skipping execution of test in short mode")
	}
}

func (helper *Helper) Count(table string) int64 {
	g2 := helper.DBFactory.New(context.Background())
	var count int64
	err := g2.Table(table).Count(&count).Error
	if err != nil {
		helper.T.Errorf("error getting count for table %s: %v", table, err)
	}
	return count
}

func (helper *Helper) MigrateDB() error {
	return db.Migrate(helper.DBFactory.New(context.Background()))
}

func (helper *Helper) MigrateDBTo(migrationID string) {
	db.MigrateTo(helper.DBFactory, migrationID)
}

func (helper *Helper) ClearAllTables() {
	helper.DeleteAll(&api.Resource{})
}

func (helper *Helper) CleanDB() error {
	g2 := helper.DBFactory.New(context.Background())

	// TODO: this list should not be static or otherwise not hard-coded here.
	for _, table := range []string{
		"events",
		"resources",
		"consumers",
		"migrations",
	} {
		if g2.Migrator().HasTable(table) {
			if err := g2.Migrator().DropTable(table); err != nil {
				helper.T.Errorf("error dropping table %s: %v", table, err)
				return err
			}
		} else {
			helper.T.Errorf("Unable to drop table %q, it does not exist", table)
		}
	}
	return nil
}

func (helper *Helper) ResetDB() error {
	if err := helper.CleanDB(); err != nil {
		return err
	}

	if err := helper.MigrateDB(); err != nil {
		return err
	}

	return nil
}

func (helper *Helper) CreateJWTString(account *amv1.Account) string {
	// Use an RH SSO JWT by default since we are phasing RHD out
	claims := jwt.MapClaims{
		"iss":        helper.Env().Config.OCM.TokenURL,
		"username":   strings.ToLower(account.Username()),
		"first_name": account.FirstName(),
		"last_name":  account.LastName(),
		"typ":        "Bearer",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(1 * time.Hour).Unix(),
	}
	if account.Email() != "" {
		claims["email"] = account.Email()
	}
	/* TODO the ocm api model needs to be updated to expose this
	if account.ServiceAccount {
		claims["clientId"] = account.Username()
	}
	*/

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// Set the token header kid to the same value we expect when validating the token
	// The kid is an arbitrary identifier for the key
	// See https://tools.ietf.org/html/rfc7517#section-4.5
	token.Header["kid"] = jwkKID

	// private key and public key taken from http://kjur.github.io/jsjws/tool_jwt.html
	// the go-jwt-middleware pkg we use does the same for their tests
	signedToken, err := token.SignedString(helper.JWTPrivateKey)
	if err != nil {
		helper.T.Errorf("Unable to sign test jwt: %s", err)
		return ""
	}
	return signedToken
}

func (helper *Helper) CreateJWTToken(account *amv1.Account) *jwt.Token {
	tokenString := helper.CreateJWTString(account)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return helper.JWTCA, nil
	})
	if err != nil {
		helper.T.Errorf("Unable to parse signed jwt: %s", err)
		return nil
	}
	return token
}

// Convert an error response from the openapi client to an openapi error struct
func (helper *Helper) OpenapiError(err error) openapi.Error {
	generic := err.(openapi.GenericOpenAPIError)
	var exErr openapi.Error
	jsonErr := json.Unmarshal(generic.Body(), &exErr)
	if jsonErr != nil {
		helper.T.Errorf("Unable to convert error response to openapi error: %s", jsonErr)
	}
	return exErr
}

func parseJWTKeys() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	projectRootDir := getProjectRootDir()
	privateBytes, err := os.ReadFile(filepath.Join(projectRootDir, jwtKeyFile))
	if err != nil {
		err = fmt.Errorf("unable to read JWT key file %s: %s", jwtKeyFile, err)
		return nil, nil, err
	}
	pubBytes, err := os.ReadFile(filepath.Join(projectRootDir, jwtCAFile))
	if err != nil {
		err = fmt.Errorf("unable to read JWT ca file %s: %s", jwtKeyFile, err)
		return nil, nil, err
	}

	// Parse keys
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEMWithPassword(privateBytes, "passwd")
	if err != nil {
		err = fmt.Errorf("unable to parse JWT private key: %s", err)
		return nil, nil, err
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		err = fmt.Errorf("unable to parse JWT ca: %s", err)
		return nil, nil, err
	}

	return privateKey, pubKey, nil
}

// Return project root path based on the relative path of this file
func getProjectRootDir() string {
	ulog := logger.NewOCMLogger(context.Background())
	curr, err := os.Getwd()
	if err != nil {
		ulog.Fatal(fmt.Sprintf("Unable to get working directory: %v", err.Error()))
		return ""
	}
	root := curr
	for {
		anchor := filepath.Join(curr, ".git")
		_, err = os.Stat(anchor)
		if err != nil && !os.IsNotExist(err) {
			ulog.Fatal(fmt.Sprintf("Unable to check if directory '%s' exists", anchor))
			break
		}
		if err == nil {
			root = curr
			break
		}
		next := filepath.Dir(curr)
		if next == curr {
			break
		}
		curr = next
	}
	return root
}
