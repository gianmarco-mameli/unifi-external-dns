package main

import (
	"context"
	"fmt"
	"log"
	"strings"
)

func syncOnce(ctx context.Context, cfg Config, dockerRecords []DNSRecord, yamlRecords []DNSRecord, client *UnifiClient, siteID string) error {
	desired := make(map[string]DNSRecord)
	for _, record := range yamlRecords {
		desired[record.Key()] = record
	}
	for _, record := range dockerRecords {
		desired[record.Key()] = record
	}
	desiredCount := len(desired)
	if cfg.TXTPrefix != "" {
		txtRecords := buildTXTRegistryRecords(cfg.TXTPrefix, desired)
		for _, txtRecord := range txtRecords {
			desired[txtRecord.Key()] = txtRecord
		}
	}

	policies, err := client.listDNSPolicies(ctx, siteID)
	if err != nil {
		return err
	}

	existing := make(map[string]dnsPolicy)
	for _, policy := range policies {
		key := policy.Type + "|" + policy.Domain
		if policy.Type == recordTypeSRV {
			key = policy.Type + "|" + policy.Service + "." + policy.Protocol + "." + policy.Domain
		}
		existing[key] = policy
	}

	created := 0
	updated := 0
	deleted := 0

	for _, record := range desired {
		policy, found := existing[record.Key()]
		if !found {
			if err := client.createDNSPolicy(ctx, siteID, record); err != nil {
				return fmt.Errorf("create %s %s: %w", record.Type, record.Domain, err)
			}
			created++
			continue
		}

		needsUpdate := false
		if record.Type == recordTypeSRV {
			needsUpdate = policy.ServerDomain != record.SrvTarget || policy.Service != record.SrvService ||
				policy.Protocol != record.SrvProtocol || policy.Port != record.SrvPort ||
				policy.Priority != record.SrvPriority || policy.Weight != record.SrvWeight || !policy.Enabled
		} else {
			value := policyValue(policy)
			needsUpdate = value != record.Value || !policy.Enabled
			if record.Type != recordTypeTXT {
				needsUpdate = needsUpdate || policy.TTLSeconds != record.TTL
			}
		}

		if needsUpdate {
			if err := client.updateDNSPolicy(ctx, siteID, policy.ID, record); err != nil {
				return fmt.Errorf("update %s %s: %w", record.Type, record.Domain, err)
			}
			updated++
		}
	}

	if cfg.PrunePolicies {
		for key, policy := range existing {
			if _, ok := desired[key]; ok {
				continue
			}
			if err := client.deleteDNSPolicy(ctx, siteID, policy.ID); err != nil {
				return fmt.Errorf("delete %s %s: %w", policy.Type, policy.Domain, err)
			}
			deleted++
		}
	}

	log.Printf("sync complete: desired=%d created=%d updated=%d deleted=%d", desiredCount, created, updated, deleted)
	return nil
}

func policyValue(policy dnsPolicy) string {
	if policy.Type == recordTypeA {
		return policy.IPv4Address
	}
	if policy.Type == recordTypeTXT {
		if policy.Text != "" {
			return policy.Text
		}
		if policy.TXT != "" {
			return policy.TXT
		}
		return policy.Value
	}
	if policy.CanonicalName != "" {
		return policy.CanonicalName
	}
	return policy.CNAME
}

func buildTXTRegistryRecords(prefix string, desired map[string]DNSRecord) []DNSRecord {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}

	records := make([]DNSRecord, 0, len(desired))
	for _, record := range desired {
		if record.Type == recordTypeTXT {
			continue
		}
		resource := record.Resource
		if strings.TrimSpace(resource) == "" {
			resource = "unknown"
		}

		txtDomain := record.Domain
		// For SRV records, remove underscores to make valid DNS names
		if record.Type == recordTypeSRV {
			txtDomain = strings.ReplaceAll(txtDomain, "_", "")
		}

		records = append(records, DNSRecord{
			Type:     recordTypeTXT,
			Domain:   prefix + "." + txtDomain,
			Value:    fmt.Sprintf("\"heritage=unifi-external-dns,unifi-external-dns/owner=%s,unifi-external-dns/resource=%s\"", prefix, resource),
			TTL:      record.TTL,
			Resource: resource,
		})
	}
	return records
}
