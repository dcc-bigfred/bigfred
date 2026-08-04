package microinit

import (
	"bytes"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

type RDBSavePoint struct{ Seconds, Changes int }

var DefaultRDBSavePoints = []RDBSavePoint{{Seconds: 60, Changes: 100}}

type RedisConfig struct {
	Bin, BindAddr, DataDir string
	Port                   uint16
	RDBSavePoints          []RDBSavePoint
	Disable                bool
}

func ParseRDBSavePoint(value string) (RDBSavePoint, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return RDBSavePoint{}, fmt.Errorf("redis rdb save %q: want seconds:changes", value)
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return RDBSavePoint{}, err
	}
	changes, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return RDBSavePoint{}, err
	}
	if seconds <= 0 || changes <= 0 {
		return RDBSavePoint{}, fmt.Errorf("redis rdb save %q: values must be > 0", value)
	}
	return RDBSavePoint{seconds, changes}, nil
}
func ParseRDBSavePoints(values []string) ([]RDBSavePoint, error) {
	var out []RDBSavePoint
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		point, err := ParseRDBSavePoint(value)
		if err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, nil
}
func ResolveRDBSavePoints(noPersist bool, values []string) ([]RDBSavePoint, error) {
	if noPersist || len(values) == 1 && strings.TrimSpace(values[0]) == "" {
		return nil, nil
	}
	if len(values) == 0 {
		return append([]RDBSavePoint(nil), DefaultRDBSavePoints...), nil
	}
	return ParseRDBSavePoints(values)
}
func RedisServiceDef(cfg RedisConfig) ServiceDef {
	bin, bind, port, dir := cfg.Bin, cfg.BindAddr, cfg.Port, cfg.DataDir
	if bin == "" {
		bin = "redis-server"
	}
	if bind == "" {
		bind = "127.0.0.1"
	}
	if port == 0 {
		port = 6379
	}
	if dir == "" {
		dir = "."
	}
	args := []string{
		shellQuote(bin),
		"--bind", shellQuote(bind),
		"--port", strconv.Itoa(int(port)),
		"--dir", shellQuote(dir),
		"--daemonize", "no",
		"--protected-mode", "no",
		"--logfile", "''",
		"--appendonly", "no",
		"--save", "''",
	}
	for _, point := range cfg.RDBSavePoints {
		// Redis expects: --save <seconds> <changes> as two argv tokens.
		args = append(args, "--save", strconv.Itoa(point.Seconds), strconv.Itoa(point.Changes))
	}
	return ServiceDef{Name: "redis", Enabled: BoolPtr(true), Daemon: BoolPtr(true), Restart: BoolPtr(true), StartWaitSecs: IntPtr(2), ShutdownWaitSecs: IntPtr(10), StartCmd: "exec " + strings.Join(args, " "), LivenessProbe: &LivenessProbe{TCPAddr: net.JoinHostPort(bind, strconv.Itoa(int(port))), Interval: 10}}
}

type TelemetryConfig struct {
	Enable                                          bool
	AlloyBin, ConfigPath, StoragePath, OTLPEndpoint string
}
type InfraConfig struct {
	Redis     RedisConfig
	Telemetry TelemetryConfig
}

const DefaultOTLPEndpoint = "127.0.0.1:4317"

func DefaultTelemetryConfigPath() string { return datadir.Path("etc", "alloy.conf") }
func DefaultAlloyStoragePath() string    { return datadir.Path("alloy") }
func AlloyRunConfigPath(cfg TelemetryConfig) string {
	if cfg.ConfigPath != "" {
		return cfg.ConfigPath
	}
	return DefaultTelemetryConfigPath()
}
func BigFredAlloyGeneratedPath(cfg TelemetryConfig) string { return AlloyRunConfigPath(cfg) }
func AlloyServiceDef(cfg TelemetryConfig) ServiceDef {
	bin, storage := cfg.AlloyBin, cfg.StoragePath
	if bin == "" {
		bin = "alloy"
	}
	if storage == "" {
		storage = DefaultAlloyStoragePath()
	}
	return ServiceDef{Name: "alloy", Enabled: BoolPtr(true), Daemon: BoolPtr(true), Restart: BoolPtr(true), StartWaitSecs: IntPtr(2), ShutdownWaitSecs: IntPtr(10), StartCmd: fmt.Sprintf("exec %s run --storage.path=%s %s", shellQuote(bin), shellQuote(storage), shellQuote(AlloyRunConfigPath(cfg))), LivenessProbe: &LivenessProbe{HTTPUrl: "http://127.0.0.1:12345/-/ready", HTTPAcceptedCodes: []int{200}, Interval: 30}}
}

const alloyTemplate = `otelcol.receiver.otlp "bigfred" {
  grpc { endpoint = "{{ . }}" }
  output { metrics = [otelcol.processor.batch.bigfred.input] }
}
otelcol.processor.batch "bigfred" { output { metrics = [otelcol.exporter.otlphttp.bigfred.input] } }
otelcol.exporter.otlphttp "bigfred" { client { endpoint = sys.env("GRAFANA_CLOUD_OTLP_ENDPOINT"); auth = otelcol.auth.basic.bigfred.handler } }
otelcol.auth.basic "bigfred" { username = sys.env("GRAFANA_CLOUD_INSTANCE_ID"); password = sys.env("GRAFANA_CLOUD_API_KEY") }
`

func PrepareAlloyTelemetry(cfg TelemetryConfig) error {
	endpoint := cfg.OTLPEndpoint
	if endpoint == "" {
		endpoint = DefaultOTLPEndpoint
	}
	var content bytes.Buffer
	if err := template.Must(template.New("alloy").Parse(alloyTemplate)).Execute(&content, endpoint); err != nil {
		return err
	}
	return writeFileAtomically(filepath.Clean(AlloyRunConfigPath(cfg)), content.Bytes())
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
