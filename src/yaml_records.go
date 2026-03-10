package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type yamlConfig struct {
	Records []yamlRecord `yaml:"records"`
}

type yamlRecord struct {
	Type   string `yaml:"type"`
	Domain string `yaml:"domain"`
	IP     string `yaml:"ip"`
	CNAME  string `yaml:"cname"`
	Value  string `yaml:"value"`
	TTL    int    `yaml:"ttl"`
}

func loadYAMLRecords(path string, defaultTTL int) ([]DNSRecord, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg yamlConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	records := make([]DNSRecord, 0, len(cfg.Records))
	for _, entry := range cfg.Records {
		value := entry.Value
		if value == "" {
			if entry.IP != "" {
				value = entry.IP
			} else if entry.CNAME != "" {
				value = entry.CNAME
			}
		}

		ttl := entry.TTL
		if ttl == 0 {
			ttl = defaultTTL
		}

		record, err := normalizeRecord(entry.Type, entry.Domain, value, ttl)
		if err != nil {
			return nil, fmt.Errorf("yaml record %s: %w", entry.Domain, err)
		}
		record.Resource = "file/" + entry.Domain
		records = append(records, record)
	}

	return records, nil
}
