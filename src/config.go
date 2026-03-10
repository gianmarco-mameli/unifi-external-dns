package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	UnifiURL      string
	APIToken      string
	SiteName      string
	InsecureTLS   bool
	SyncInterval  time.Duration
	DefaultTTL    int
	PrunePolicies bool
	DNSYAMLPath   string
	TXTPrefix     string
}

func loadConfig() (Config, error) {
	cfg := Config{
		UnifiURL:     strings.TrimSpace(os.Getenv("UNIFI_URL")),
		APIToken:     strings.TrimSpace(os.Getenv("UNIFI_API_TOKEN")),
		SiteName:     strings.TrimSpace(os.Getenv("UNIFI_SITE_NAME")),
		InsecureTLS:  parseBool(os.Getenv("UNIFI_INSECURE_SKIP_VERIFY")),
		SyncInterval: parseDuration(os.Getenv("UNIFI_SYNC_INTERVAL"), 60*time.Second),
		DefaultTTL:   parseInt(os.Getenv("UNIFI_DNS_TTL_SECONDS"), 14400),
		PrunePolicies: parseBool(os.Getenv("UNIFI_DNS_PRUNE")),
		DNSYAMLPath:  strings.TrimSpace(os.Getenv("UNIFI_DNS_YAML_PATH")),
		TXTPrefix:    strings.TrimSpace(os.Getenv("UNIFI_TXT_PREFIX")),
	}

	if cfg.UnifiURL == "" {
		return cfg, errors.New("UNIFI_URL is required")
	}
	if cfg.APIToken == "" {
		return cfg, errors.New("UNIFI_API_TOKEN is required")
	}
	if cfg.SiteName == "" {
		return cfg, errors.New("UNIFI_SITE_NAME is required")
	}
	return cfg, nil
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func parseInt(value string, def int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return def
	}
	return parsed
}

func parseDuration(value string, def time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return def
	}
	return parsed
}

func normalizeRecord(recordType, domain, value string, ttl int) (DNSRecord, error) {
	recordType = strings.TrimSpace(strings.ToUpper(recordType))
	if strings.HasSuffix(recordType, "_RECORD") {
		// already normalized
	} else if recordType == "A" {
		recordType = recordTypeA
	} else if recordType == "CNAME" {
		recordType = recordTypeCNAME
	} else if recordType == "TXT" {
		recordType = recordTypeTXT
	} else if recordType == "SRV" {
		recordType = recordTypeSRV
	}

	domain = strings.TrimSpace(domain)
	value = strings.TrimSpace(value)
	if domain == "" || value == "" {
		return DNSRecord{}, fmt.Errorf("domain and value are required")
	}
	if recordType != recordTypeA && recordType != recordTypeCNAME && recordType != recordTypeTXT && recordType != recordTypeSRV {
		return DNSRecord{}, fmt.Errorf("unsupported record type: %s", recordType)
	}
	return DNSRecord{
		Type:   recordType,
		Domain: domain,
		Value:  value,
		TTL:    ttl,
	}, nil
}

func buildSRVRecord(domain, baseDomain, service, protocol, target string, port, priority, weight, ttl int) (DNSRecord, error) {
	service = strings.TrimSpace(service)
	protocol = strings.TrimSpace(protocol)
	target = strings.TrimSpace(target)
	baseDomain = strings.TrimSpace(baseDomain)

	if service == "" || protocol == "" || target == "" || baseDomain == "" || port == 0 {
		return DNSRecord{}, fmt.Errorf("SRV requires service, protocol, target, domain, and port")
	}

	if !strings.HasPrefix(service, "_") {
		service = "_" + service
	}
	if !strings.HasPrefix(protocol, "_") {
		protocol = "_" + protocol
	}

	return DNSRecord{
		Type:        recordTypeSRV,
		Domain:      baseDomain,
		Value:       target,
		TTL:         ttl,
		SrvTarget:   target,
		SrvService:  service,
		SrvProtocol: protocol,
		SrvPort:     port,
		SrvPriority: priority,
		SrvWeight:   weight,
	}, nil
}

func (r DNSRecord) FullDomain() string {
	if r.Type == recordTypeSRV {
		return r.SrvService + "." + r.SrvProtocol + "." + r.Domain
	}
	return r.Domain
}
