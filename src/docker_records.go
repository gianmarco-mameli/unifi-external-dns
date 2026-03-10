package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func loadDockerRecords(ctx context.Context, cli *client.Client, defaultTTL int) ([]DNSRecord, error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	records := make([]DNSRecord, 0)
	for _, c := range containers {
		labels := c.Labels
		containerNameStr := containerName(c.Names)

		indexedRecords, err := parseIndexedLabels(labels, defaultTTL)
		if err != nil {
			return nil, fmt.Errorf("container %s: %w", c.ID, err)
		}
		for i := range indexedRecords {
			indexedRecords[i].Resource = "docker/" + containerNameStr
		}
		records = append(records, indexedRecords...)

		if len(indexedRecords) == 0 {
			domain := labels[labelDomain]
			recordType := labels[labelType]
			value := labels[labelValue]
			if domain == "" || recordType == "" || value == "" {
				continue
			}

			ttl := defaultTTL
			if ttlLabel := labels[labelTTL]; ttlLabel != "" {
				if parsed, err := strconv.Atoi(ttlLabel); err == nil {
					ttl = parsed
				}
			}

			record, err := normalizeRecord(recordType, domain, value, ttl)
			if err != nil {
				return nil, fmt.Errorf("container %s: %w", c.ID, err)
			}
			record.Resource = "docker/" + containerNameStr
			records = append(records, record)
		}
	}

	return records, nil
}

func parseIndexedLabels(labels map[string]string, defaultTTL int) ([]DNSRecord, error) {
	re := regexp.MustCompile(`^unifi\.dns\.(\d+)\.(.+)$`)

	indexMap := make(map[string]map[string]string)

	for key, val := range labels {
		matches := re.FindStringSubmatch(key)
		if matches == nil {
			continue
		}

		index := matches[1]
		field := matches[2]

		if indexMap[index] == nil {
			indexMap[index] = make(map[string]string)
		}
		indexMap[index][field] = val
	}

	records := make([]DNSRecord, 0)
	for _, fieldMap := range indexMap {
		recordType := fieldMap["type"]
		if recordType == "" {
			continue
		}

		recordType = strings.ToUpper(recordType)
		if !strings.HasSuffix(recordType, "_RECORD") {
			switch recordType {
				case "A":
					recordType = recordTypeA
				case "CNAME":
					recordType = recordTypeCNAME
				case "TXT":
					recordType = recordTypeTXT
				case "SRV":
					recordType = recordTypeSRV
			}
		}

		ttl := defaultTTL
		if ttlStr := fieldMap["ttl"]; ttlStr != "" {
			if parsed, err := strconv.Atoi(ttlStr); err == nil {
				ttl = parsed
			}
		}

		if recordType == recordTypeSRV {
			domain := fieldMap["domain"]
			service := fieldMap["service"]
			protocol := fieldMap["protocol"]
			target := fieldMap["server"]
			portStr := fieldMap["port"]
			priorityStr := fieldMap["priority"]
			weightStr := fieldMap["weight"]

			if domain == "" || service == "" || protocol == "" || target == "" || portStr == "" {
				continue
			}

			port, err := strconv.Atoi(portStr)
			if err != nil {
				continue
			}

			priority := 0
			if priorityStr != "" {
				if p, err := strconv.Atoi(priorityStr); err == nil {
					priority = p
				}
			}

			weight := 0
			if weightStr != "" {
				if w, err := strconv.Atoi(weightStr); err == nil {
					weight = w
				}
			}

			record, err := buildSRVRecord(domain, domain, service, protocol, target, port, priority, weight, ttl)
			if err != nil {
				continue
			}
			records = append(records, record)
		} else {
			domain := fieldMap["domain"]
			value := fieldMap["value"]

			if domain == "" || value == "" {
				continue
			}

			record, err := normalizeRecord(recordType, domain, value, ttl)
			if err != nil {
				continue
			}
			records = append(records, record)
		}
	}

	return records, nil
}

func containerName(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}
	name := strings.TrimSpace(names[0])
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return "unknown"
	}
	return name
}

