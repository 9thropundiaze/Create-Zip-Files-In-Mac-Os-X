package dns

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/likexian/doh"
	dohDns "github.com/likexian/doh/dns"
	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
	"github.com/xvzc/SpoofDPI/util"
)

type DnsResolver struct {
	host      string
	port      string
	enableDoh bool
}

func NewResolver(config *util.Config) *DnsResolver {
	return &DnsResolver{
		host:      *config.DnsAddr,
		port:      strconv.Itoa(*config.DnsPort),
		enableDoh: *config.EnableDoh,
	}
}

func (d *DnsResolver) Lookup(domain string) (string, error) {
	ipRegex := "^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$"

	if r, _ := regexp.MatchString(ipRegex, domain); r {
		return domain, nil
	}

	if d.enableDoh {
		return dohLookup(domain)
	}

	dnsServer := d.host + ":" + d.port

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	c := new(dns.Client)

	response, _, err := c.Exchange(msg, dnsServer)
	if err != nil {
		return "", errors.New("couldn not resolve the domain")
	}

	for _, answer := range response.Answer {
		if record, ok := answer.(*dns.A); ok {
			log.Debug("[DNS] resolved ", domain, ": ", record.A.String())
			return record.A.String(), nil
		}
	}

	return "", errors.New("no record found")
}

func dohLookup(domain string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := doh.Use(doh.CloudflareProvider, doh.GoogleProvider)

	rsp, err := c.Query(ctx, dohDns.Domain(domain), dohDns.TypeA)
	if err != nil {
		return "", errors.New("could not resolve the domain")
	}
	// doh dns answer
	answer := rsp.Answer

	// print all answer
	for _, a := range answer {
		if a.Type != 1 { // Type == 1 -> A Record
			continue
		}

		log.Debug("[DOH] resolved ", domain, ": ", a.Data)
		return a.Data, nil
	}

	// close the client
	c.Close()

	return "", errors.New("no record found")
}
