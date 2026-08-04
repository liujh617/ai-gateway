package config

// TelemetryConfig holds telemetry configuration
type TelemetryConfig struct {
	Enabled        bool              `json:"enabled"`
	ServiceName    string            `json:"service_name"`
	ServiceVersion string            `json:"service_version"`
	Tracing        TracingConfig     `json:"tracing"`
	Metrics        MetricsConfig     `json:"metrics"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	Enabled     bool              `json:"enabled"`
	SampleRate  float64           `json:"sample_rate"`
	Exporter    string            `json:"exporter"`
	Endpoint    string            `json:"endpoint"`
	Insecure    bool              `json:"insecure"`
	Headers     map[string]string `json:"headers"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled    bool     `json:"enabled"`
	Path       string   `json:"path"`
	AllowedIPs []string `json:"allowed_ips"`
}

// Validate validates telemetry configuration
func (c *TelemetryConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Validate tracing
	if c.Tracing.Enabled {
		if c.Tracing.SampleRate < 0 || c.Tracing.SampleRate > 1 {
			return fmt.Errorf("tracing sample_rate must be between 0 and 1")
		}
		if c.Tracing.Exporter == "" {
			c.Tracing.Exporter = "otlp"
		}
		if c.Tracing.Exporter == "otlp" && c.Tracing.Endpoint == "" {
			return fmt.Errorf("tracing endpoint is required for otlp exporter")
		}
	}

	// Validate metrics
	if c.Metrics.Enabled {
		if c.Metrics.Path == "" {
			c.Metrics.Path = "/metrics"
		}
	}

	return nil
}

// ApplyEnvironmentOverrides applies environment variable overrides
func (c *TelemetryConfig) ApplyEnvironmentOverrides() {
	if enabled := os.Getenv("AI_GATEWAY_TELEMETRY_ENABLED"); enabled != "" {
		c.Enabled = enabled == "true"
	}

	if serviceName := os.Getenv("AI_GATEWAY_SERVICE_NAME"); serviceName != "" {
		c.ServiceName = serviceName
	}

	if serviceVersion := os.Getenv("AI_GATEWAY_SERVICE_VERSION"); serviceVersion != "" {
		c.ServiceVersion = serviceVersion
	}

	if tracingEnabled := os.Getenv("AI_GATEWAY_TRACING_ENABLED"); tracingEnabled != "" {
		c.Tracing.Enabled = tracingEnabled == "true"
	}

	if sampleRate := os.Getenv("AI_GATEWAY_TRACING_SAMPLE_RATE"); sampleRate != "" {
		if rate, err := strconv.ParseFloat(sampleRate, 64); err == nil {
			c.Tracing.SampleRate = rate
		}
	}

	if endpoint := os.Getenv("AI_GATEWAY_TRACING_ENDPOINT"); endpoint != "" {
		c.Tracing.Endpoint = endpoint
	}

	if exporter := os.Getenv("AI_GATEWAY_TRACING_EXPORTER"); exporter != "" {
		c.Tracing.Exporter = exporter
	}

	if metricsEnabled := os.Getenv("AI_GATEWAY_METRICS_ENABLED"); metricsEnabled != "" {
		c.Metrics.Enabled = metricsEnabled == "true"
	}

	if metricsPath := os.Getenv("AI_GATEWAY_METRICS_PATH"); metricsPath != "" {
		c.Metrics.Path = metricsPath
	}
}