package server

import (
	"github.com/Bogh/gcm"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"

	"github.com/cosminrentea/gobbler/server/apns"
	"github.com/cosminrentea/gobbler/server/fcm"
	"github.com/cosminrentea/gobbler/server/sms"
	"github.com/cosminrentea/gobbler/server/websocket"
)

const (
	defaultHttpListen      = ":8080"
	defaultHealthEndpoint  = "/admin/healthcheck"
	defaultMetricsEndpoint = "/admin/metrics"
	defaultKVSBackend      = "file"
	defaultMSBackend       = "file"
	defaultStoragePath     = "/var/lib/guble"
	defaultNodePort        = "10000"
	development            = "dev"
	integration            = "int"
	preproduction          = "pre"
	production             = "prod"
	memProfile             = "mem"
	cpuProfile             = "cpu"
	blockProfile           = "block"
)

var (
	defaultFCMEndpoint = gcm.GcmSendEndpoint
	defaultFCMMetrics  = true
	defaultAPNSMetrics = true
	defaultSMSMetrics  = true
	environments       = []string{development, integration, preproduction, production}
)

type (
	// PostgresConfig is used for configuring the Postgresql connection.
	PostgresConfig struct {
		Host     *string
		Port     *int
		User     *string
		Password *string
		DbName   *string
	}
	// ClusterConfig is used for configuring the cluster component.
	ClusterConfig struct {
		NodeID   *uint8
		NodePort *int
		Remotes  *tcpAddrList
	}
	// GubleConfig is used for configuring Guble server (including its modules / connectors).
	GubleConfig struct {
		Log             *string
		EnvName         *string
		HttpListen      *string
		KVS             *string
		MS              *string
		StoragePath     *string
		HealthEndpoint  *string
		MetricsEndpoint *string
		Profile         *string
		Postgres        PostgresConfig
		FCM             fcm.Config
		APNS            apns.Config
		SMS             sms.Config
		WS		websocket.Config
		Cluster         ClusterConfig
	}
)

var (
	parsed = false

	// Config is the active configuration of guble (used when starting-up the server)
	Config = &GubleConfig{
		Log: kingpin.Flag("log", "Log level").
			Default(log.ErrorLevel.String()).
			Envar("GUBLE_LOG").
			Enum(logLevels()...),
		EnvName: kingpin.Flag("env", `Name of the environment on which the application is running`).
			Default(development).
			Envar("GUBLE_ENV").
			Enum(environments...),
		HttpListen: kingpin.Flag("http", `The address to for the HTTP server to listen on (format: "[Host]:Port")`).
			Default(defaultHttpListen).
			Envar("GUBLE_HTTP_LISTEN").
			String(),
		KVS: kingpin.Flag("kvs", "The storage backend for the key-value store to use : file | memory | postgres ").
			Default(defaultKVSBackend).
			Envar("GUBLE_KVS").
			String(),
		MS: kingpin.Flag("ms", "The message storage backend : file | memory").
			Default(defaultMSBackend).
			HintOptions("file", "memory").
			Envar("GUBLE_MS").
			String(),
		StoragePath: kingpin.Flag("storage-path", "The path for storing messages and key-value data if 'file' is selected").
			Default(defaultStoragePath).
			Envar("GUBLE_STORAGE_PATH").
			ExistingDir(),
		HealthEndpoint: kingpin.Flag("health-endpoint", `The health endpoint to be used by the HTTP server (value for disabling it: "")`).
			Default(defaultHealthEndpoint).
			Envar("GUBLE_HEALTH_ENDPOINT").
			String(),
		MetricsEndpoint: kingpin.Flag("metrics-endpoint", `The metrics endpoint to be used by the HTTP server (value for disabling it: "")`).
			Default(defaultMetricsEndpoint).
			Envar("GUBLE_METRICS_ENDPOINT").
			String(),
		Profile: kingpin.Flag("profile", `The profiler to be used (default: none): mem | cpu | block`).
			Default("").
			Envar("GUBLE_PROFILE").
			Enum("mem", "cpu", "block", ""),
		Postgres: PostgresConfig{
			Host: kingpin.Flag("pg-host", "The PostgreSQL hostname").
				Default("localhost").
				Envar("GUBLE_PG_HOST").
				String(),
			Port: kingpin.Flag("pg-port", "The PostgreSQL port").
				Default("5432").
				Envar("GUBLE_PG_PORT").
				Int(),
			User: kingpin.Flag("pg-user", "The PostgreSQL user").
				Default("guble").
				Envar("GUBLE_PG_USER").
				String(),
			Password: kingpin.Flag("pg-password", "The PostgreSQL password").
				Default("guble").
				Envar("GUBLE_PG_PASSWORD").
				String(),
			DbName: kingpin.Flag("pg-dbname", "The PostgreSQL database name").
				Default("guble").
				Envar("GUBLE_PG_DBNAME").
				String(),
		},
		FCM: fcm.Config{
			Enabled: kingpin.Flag("fcm", "Enable the Google Firebase Cloud Messaging connector").
				Envar("GUBLE_FCM").
				Bool(),
			APIKey: kingpin.Flag("fcm-api-key", "The Google API Key for Google Firebase Cloud Messaging").
				Envar("GUBLE_FCM_API_KEY").
				String(),
			Workers: kingpin.Flag("fcm-workers", "The number of workers handling traffic with Firebase Cloud Messaging (default: number of CPUs)").
				Default(strconv.Itoa(runtime.NumCPU())).
				Envar("GUBLE_FCM_WORKERS").
				Int(),
			Endpoint: kingpin.Flag("fcm-endpoint", "The Google Firebase Cloud Messaging endpoint").
				Default(defaultFCMEndpoint).
				Envar("GUBLE_FCM_ENDPOINT").
				String(),
			Prefix: kingpin.Flag("fcm-prefix", "The FCM prefix / endpoint").
				Envar("GUBLE_FCM_PREFIX").
				Default("/fcm/").
				String(),
			IntervalMetrics: &defaultFCMMetrics,
		},
		APNS: apns.Config{
			Enabled: kingpin.Flag("apns", "Enable the APNS connector (by default, in Development mode)").
				Envar("GUBLE_APNS").
				Bool(),
			Production: kingpin.Flag("apns-production", "Enable the APNS connector in Production mode").
				Envar("GUBLE_APNS_PRODUCTION").
				Bool(),
			CertificateFileName: kingpin.Flag("apns-cert-file", "The APNS certificate file name").
				Envar("GUBLE_APNS_CERT_FILE").
				String(),
			CertificateBytes: kingpin.Flag("apns-cert-bytes", "The APNS certificate bytes, as a string of hex-values").
				Envar("GUBLE_APNS_CERT_BYTES").
				HexBytes(),
			CertificatePassword: kingpin.Flag("apns-cert-password", "The APNS certificate password").
				Envar("GUBLE_APNS_CERT_PASSWORD").
				String(),
			AppTopic: kingpin.Flag("apns-app-topic", "The APNS topic (as used by the mobile application)").
				Envar("GUBLE_APNS_APP_TOPIC").
				String(),
			Prefix: kingpin.Flag("apns-prefix", "The APNS prefix / endpoint").
				Envar("GUBLE_APNS_PREFIX").
				Default("/apns/").
				String(),
			Workers: kingpin.Flag("apns-workers", "The number of workers handling traffic with APNS (default: number of CPUs)").
				Default(strconv.Itoa(runtime.NumCPU())).
				Envar("GUBLE_APNS_WORKERS").
				Int(),
			IntervalMetrics: &defaultAPNSMetrics,
		},
		Cluster: ClusterConfig{
			NodeID: kingpin.Flag("node-id", "(cluster mode) This guble node's own ID: a strictly positive integer number which must be unique in cluster").
				Envar("GUBLE_NODE_ID").Uint8(),
			NodePort: kingpin.Flag("node-port", "(cluster mode) This guble node's own local port: a strictly positive integer number").
				Default(defaultNodePort).Envar("GUBLE_NODE_PORT").Int(),
			Remotes: tcpAddrListParser(kingpin.Flag("remotes", `(cluster mode) The list of TCP addresses of some other guble nodes (format: "IP:port")`).
				Envar("GUBLE_NODE_REMOTES")),
		},
		SMS: sms.Config{
			Enabled: kingpin.Flag("sms", "Enable the SMS gateway").
				Envar("GUBLE_SMS").
				Bool(),
			APIKey: kingpin.Flag("sms-api-key", "The Nexmo API Key for Sending sms").
				Envar("GUBLE_SMS_API_KEY").
				String(),
			APISecret: kingpin.Flag("sms-api-secret", "The Nexmo API Secret for Sending sms").
				Envar("GUBLE_SMS_API_SECRET").
				String(),
			SMSTopic: kingpin.Flag("sms-topic", "The topic for sms route").
				Envar("GUBLE_SMS_TOPIC").
				Default(sms.SMSDefaultTopic).
				String(),
			Workers: kingpin.Flag("sms-workers", "The number of workers handling traffic with Nexmo sms endpoint(default: number of CPUs)").
				Default(strconv.Itoa(runtime.NumCPU())).
				Envar("GUBLE_SMS_WORKERS").
				Int(),
			IntervalMetrics: &defaultSMSMetrics,
		},
		WS: websocket.Config{
			Enabled: kingpin.Flag("ws", "Enable the websocket module").
				Envar("GUBLE_WS").
				Bool(),
			Prefix: kingpin.Flag("ws-prefix", "The Websocket prefix").
				Envar("GUBLE_WS_PREFIX").
				Default("/stream/").
				String(),
		},
	}
)

func logLevels() (levels []string) {
	for _, level := range log.AllLevels {
		levels = append(levels, level.String())
	}
	return
}

// parseConfig parses the flags from command line. Must be used before accessing the config.
// If there are missing or invalid arguments it will exit the application
// and display a message.
func parseConfig() {
	if parsed {
		return
	}
	kingpin.Parse()
	parsed = true
	return
}

type tcpAddrList []*net.TCPAddr

func (h *tcpAddrList) Set(value string) error {
	addresses := strings.Split(value, " ")

	// Reset the list also, when running tests we add to the same list and is incorrect
	*h = make(tcpAddrList, 0)
	for _, addr := range addresses {
		logger.WithField("addr", addr).Info("value")
		parts := strings.SplitN(addr, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("expected HEADER:VALUE got '%s'", addr)
		}
		addr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return err
		}
		*h = append(*h, addr)
	}
	return nil
}

func tcpAddrListParser(s kingpin.Settings) (target *tcpAddrList) {
	slist := make(tcpAddrList, 0)
	s.SetValue(&slist)
	return &slist
}

func (h *tcpAddrList) String() string {
	return ""
}
