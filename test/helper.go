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
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/dispatcher"
	"github.com/openshift-online/maestro/pkg/event"
	"github.com/openshift-online/maestro/pkg/logger"
	"k8s.io/klog/v2"

	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	workv1informers "open-cluster-management.io/api/client/work/informers/externalversions/work/v1"

	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic"
	grpcoptions "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/mqtt"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/agent/codec"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/work/store"

	"github.com/bxcodec/faker/v3"
	"github.com/golang-jwt/jwt/v4"
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
	Ctx               context.Context
	ContextCancelFunc context.CancelFunc

	Broker            string
	EventBroadcaster  *event.EventBroadcaster
	Store             *MemoryStore
	GRPCSourceClient  *generic.CloudEventSourceClient[*api.Resource]
	DBFactory         db.SessionFactory
	AppConfig         *config.ApplicationConfig
	APIServer         server.Server
	MetricsServer     server.Server
	HealthCheckServer *server.HealthCheckServer
	StatusDispatcher  dispatcher.Dispatcher
	EventServer       server.EventServer
	EventFilter       controllers.EventFilter
	ControllerManager *server.ControllersServer
	WorkAgentHolder   *work.ClientHolder
	WorkAgentInformer workv1informers.ManifestWorkInformer
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

		env := helper.Env()
		// Manually set environment name, ignoring environment variables
		env.Name = environments.TestingEnv
		err = env.AddFlags(pflag.CommandLine)
		if err != nil {
			klog.Fatalf("Unable to add environment flags: %s", err.Error())
		}
		if logLevel := os.Getenv("LOGLEVEL"); logLevel != "" {
			klog.Infof("Using custom loglevel: %s", logLevel)
			pflag.CommandLine.Set("-v", logLevel)
		}
		pflag.Parse()

		// Set the message broker type if it is set in the environment
		broker := os.Getenv("BROKER")
		if broker != "" {
			env.Config.MessageBroker.MessageBrokerType = broker
		}

		err = env.Initialize()
		if err != nil {
			klog.Fatalf("Unable to initialize testing environment: %s", err.Error())
		}

		ctx, cancel := context.WithCancel(context.Background())
		helper = &Helper{
			Ctx:               ctx,
			ContextCancelFunc: cancel,
			Broker:            env.Config.MessageBroker.MessageBrokerType,
			EventBroadcaster:  event.NewEventBroadcaster(),
			AppConfig:         env.Config,
			DBFactory:         env.Database.SessionFactory,
			JWTPrivateKey:     jwtKey,
			JWTCA:             jwtCA,
		}

		// Set the healthcheck interval to 1 second for testing
		helper.Env().Config.HealthCheck.HeartbeartInterval = 1
		helper.HealthCheckServer = server.NewHealthCheckServer()
		// Disable TLS for testing
		helper.Env().Config.GRPCServer.DisableTLS = true

		if helper.Broker != "grpc" {
			helper.StatusDispatcher = dispatcher.NewHashDispatcher(
				helper.Env().Config.MessageBroker.ClientID,
				helper.Env().Database.SessionFactory,
				helper.Env().Clients.CloudEventsSource,
				helper.Env().Config.EventServer.ConsistentHashConfig,
			)
			helper.EventServer = server.NewMessageQueueEventServer(helper.EventBroadcaster, helper.StatusDispatcher)
			helper.EventFilter = controllers.NewLockBasedEventFilter(db.NewAdvisoryLockFactory(helper.Env().Database.SessionFactory))
		} else {
			helper.EventServer = server.NewGRPCBroker(helper.EventBroadcaster)
			helper.EventFilter = controllers.NewPredicatedEventFilter(helper.EventServer.PredicateEvent)
		}

		// TODO jwk mock server needs to be refactored out of the helper and into the testing environment
		jwkMockTeardown := helper.StartJWKCertServerMock()
		helper.teardowns = []func() error{
			helper.sendShutdownSignal,
			helper.stopAPIServer,
			helper.stopMetricsServer,
			jwkMockTeardown,
		}

		if err := helper.MigrateDB(); err != nil {
			panic(err)
		}

		helper.startEventBroadcaster()
		helper.startAPIServer()
		helper.startMetricsServer()
		helper.startHealthCheckServer(helper.Ctx)
		helper.startEventServer(helper.Ctx)
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
	helper.APIServer = server.NewAPIServer(helper.EventBroadcaster)
	go func() {
		klog.V(10).Info("Test API server started")
		helper.APIServer.Start()
		klog.V(10).Info("Test API server stopped")
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
		klog.V(10).Info("Test Metrics server started")
		helper.MetricsServer.Start()
		klog.V(10).Info("Test Metrics server stopped")
	}()
}

func (helper *Helper) stopMetricsServer() error {
	if err := helper.MetricsServer.Stop(); err != nil {
		return fmt.Errorf("unable to stop metrics server: %s", err.Error())
	}
	return nil
}

func (helper *Helper) startHealthCheckServer(ctx context.Context) {
	go func() {
		klog.V(10).Info("Test health check server started")
		helper.HealthCheckServer.Start(ctx)
		klog.V(10).Info("Test health check server stopped")
	}()
}

func (helper *Helper) sendShutdownSignal() error {
	helper.ContextCancelFunc()
	return nil
}

func (helper *Helper) startEventServer(ctx context.Context) {
	go func() {
		klog.V(10).Info("Test event server started")
		helper.EventServer.Start(ctx)
		klog.V(10).Info("Test event server stopped")
	}()
}

func (helper *Helper) startEventBroadcaster() {
	go func() {
		klog.V(10).Info("Test event broadcaster started")
		helper.EventBroadcaster.Start(helper.Ctx)
		klog.V(10).Info("Test event broadcaster stopped")
	}()
}

func (helper *Helper) StartControllerManager(ctx context.Context) {
	helper.ControllerManager = &server.ControllersServer{
		KindControllerManager: controllers.NewKindControllerManager(
			helper.EventFilter,
			helper.Env().Services.Events(),
		),
		StatusController: controllers.NewStatusController(
			helper.Env().Services.StatusEvents(),
			dao.NewInstanceDao(&helper.Env().Database.SessionFactory),
			dao.NewEventInstanceDao(&helper.Env().Database.SessionFactory),
		),
	}

	helper.ControllerManager.KindControllerManager.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {helper.EventServer.OnCreate},
			api.UpdateEventType: {helper.EventServer.OnUpdate},
			api.DeleteEventType: {helper.EventServer.OnDelete},
		},
	})
	helper.ControllerManager.StatusController.Add(map[api.StatusEventType][]controllers.StatusHandlerFunc{
		api.StatusUpdateEventType: {helper.EventServer.OnStatusUpdate},
		api.StatusDeleteEventType: {helper.EventServer.OnStatusUpdate},
	})

	// start controller manager
	go helper.ControllerManager.Start(ctx)
}

func (helper *Helper) StartWorkAgent(ctx context.Context, clusterName string) {
	var brokerConfig any
	if helper.Broker != "grpc" {
		// initilize the mqtt options
		mqttOptions, err := mqtt.BuildMQTTOptionsFromFlags(helper.Env().Config.MessageBroker.MessageBrokerConfig)
		if err != nil {
			klog.Fatalf("Unable to build MQTT options: %s", err.Error())
		}
		brokerConfig = mqttOptions
	} else {
		// initilize the grpc options
		grpcOptions := grpcoptions.NewGRPCOptions()
		grpcOptions.URL = fmt.Sprintf("%s:%s", helper.Env().Config.HTTPServer.Hostname, helper.Env().Config.GRPCServer.BrokerBindPort)
		brokerConfig = grpcOptions
	}

	watcherStore := store.NewAgentInformerWatcherStore()

	clientHolder, err := work.NewClientHolderBuilder(brokerConfig).
		WithClientID(clusterName).
		WithClusterName(clusterName).
		WithCodecs(codec.NewManifestBundleCodec()).
		WithWorkClientWatcherStore(watcherStore).
		NewAgentClientHolder(ctx)
	if err != nil {
		klog.Fatalf("Unable to create work agent holder: %s", err)
	}

	factory := workinformers.NewSharedInformerFactoryWithOptions(
		clientHolder.WorkInterface(),
		5*time.Minute,
		workinformers.WithNamespace(clusterName),
	)
	informer := factory.Work().V1().ManifestWorks()
	watcherStore.SetInformer(informer.Informer())

	go informer.Informer().Run(ctx.Done())

	helper.WorkAgentHolder = clientHolder
	helper.WorkAgentInformer = informer
}

func (helper *Helper) StartGRPCResourceSourceClient() {
	store := NewStore()
	grpcOptions := grpcoptions.NewGRPCOptions()
	grpcOptions.URL = fmt.Sprintf("%s:%s", helper.Env().Config.HTTPServer.Hostname, helper.Env().Config.GRPCServer.ServerBindPort)
	sourceClient, err := generic.NewCloudEventSourceClient[*api.Resource](
		helper.Ctx,
		grpcoptions.NewSourceOptions(grpcOptions, "maestro"),
		store,
		resourceStatusHashGetter,
		&ResourceCodec{},
	)

	if err != nil {
		klog.Fatalf("Unable to create grpc cloudevents source client: %s", err.Error())
	}

	sourceClient.Subscribe(helper.Ctx, func(action types.ResourceAction, resource *api.Resource) error {
		return store.UpdateStatus(resource)
	})

	helper.Store = store
	helper.GRPCSourceClient = sourceClient
}

func (helper *Helper) RestartServer() {
	helper.stopAPIServer()
	helper.startAPIServer()
	klog.V(10).Info("Test API server restarted")
}

func (helper *Helper) RestartMetricsServer() {
	helper.stopMetricsServer()
	helper.startMetricsServer()
	klog.V(10).Info("Test metrics server restarted")
}

func (helper *Helper) Reset() {
	klog.Infof("Reseting testing environment")
	env := helper.Env()
	// Reset the configuration
	env.Config = config.NewApplicationConfig()

	// Re-read command-line configuration into a NEW flagset
	// This new flag set ensures we don't hit conflicts defining the same flag twice
	// Also on reset, we don't care to be re-defining 'v' and other klog flags
	flagset := pflag.NewFlagSet(helper.NewID(), pflag.ContinueOnError)
	env.AddFlags(flagset)
	pflag.Parse()

	err := env.Initialize()
	if err != nil {
		klog.Fatalf("Unable to reset testing environment: %s", err.Error())
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
	return fmt.Sprintf("%s://%s:%s/api/maestro/v1%s", protocol, helper.AppConfig.HTTPServer.Hostname,
		helper.AppConfig.HTTPServer.BindPort, path)
}

func (helper *Helper) MetricsURL(path string) string {
	return fmt.Sprintf("http://%s:%s%s", helper.AppConfig.HTTPServer.Hostname, helper.AppConfig.Metrics.BindPort, path)
}

func (helper *Helper) HealthCheckURL(path string) string {
	return fmt.Sprintf("http://%s:%s%s", helper.AppConfig.HTTPServer.Hostname, helper.AppConfig.HealthCheck.BindPort, path)
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
		"status_events",
		"resources",
		"consumers",
		"server_instances",
	} {
		if g2.Migrator().HasTable(table) {
			// remove table contents instead of dropping table
			sql := fmt.Sprintf("DELETE FROM %s", table)
			if err := g2.Exec(sql).Error; err != nil {
				helper.T.Errorf("error delete content of table %s: %v", table, err)
				return err
			}
		}
	}
	return nil
}

func (helper *Helper) ResetDB() error {
	if err := helper.CleanDB(); err != nil {
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
