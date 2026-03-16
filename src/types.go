package main

const (
	labelDomain = "unifi.dns.domain"
	labelType   = "unifi.dns.type"
	labelValue  = "unifi.dns.value"
	labelTTL    = "unifi.dns.ttl"
)

const (
	recordTypeA     = "A_RECORD"
	recordTypeCNAME = "CNAME_RECORD"
	recordTypeTXT   = "TXT_RECORD"
	recordTypeSRV   = "SRV_RECORD"
)

type DNSRecord struct {
	Type        string
	Domain      string
	Value       string
	TTL         int
	Resource    string
	SrvTarget   string
	SrvService  string
	SrvProtocol string
	SrvPort     int
	SrvPriority int
	SrvWeight   int
}

func (r DNSRecord) Key() string {
	if r.Type == recordTypeSRV {
		return r.Type + "|" + r.SrvService + "." + r.SrvProtocol + "." + r.Domain + "|" + r.SrvTarget
	}
	return r.Type + "|" + r.Domain
}
