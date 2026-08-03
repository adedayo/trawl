package scanner

import (
    "context"
    "fmt"
    "net"
    "strings"
    "time"
    "encoding/hex"

    "github.com/miekg/dns"
    // custom SPF lookup via DNS TXT records
    // custom DKIM lookup via DNS TXT records
    // custom DMARC lookup via DNS TXT records
)

// EmailPosture holds audit results for a domain.
// Fields correspond to the database schema defined in pkg/store/store.go.
type EmailPosture struct {
    Domain      string `json:"domain"`
    SPFResult   string `json:"spfResult"`
    DKIMResult  string `json:"dkimResult"`
    DMARCResult string `json:"dmarcResult"`
    MTASTS      string `json:"mtaSts"`
    DNSSEC      string `json:"dnssec"`
    DANE        string `json:"dane"`
}

// ScanDomain performs a comprehensive email security audit for the given domain.
// It returns an EmailPosture populated with SPF, DKIM, DMARC, MTA-STS, DNSSEC and DANE results.
func ScanDomain(ctx context.Context, domain string) (*EmailPosture, error) {
    // Set a timeout for the entire scan.
    var cancel context.CancelFunc
    ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
    defer cancel()

    result := &EmailPosture{Domain: domain}

    // SPF
    if spfRecord, err := lookupSPF(domain); err == nil {
        result.SPFResult = spfRecord
    } else {
        result.SPFResult = "error: " + err.Error()
    }

        // DKIM (using a generic selector "default" – callers can extend this).
    if dkimRecord, err := lookupDKIM(domain, "default"); err == nil {
        result.DKIMResult = dkimRecord
    } else {
        result.DKIMResult = "error: " + err.Error()
    }

        // DMARC
    if dmarcRecord, err := lookupDMARC(domain); err == nil {
        result.DMARCResult = dmarcRecord
    } else {
        result.DMARCResult = "error: " + err.Error()
    }

    // MTA-STS – a simple existence check of the _mta-sts.<domain> TXT record.
    if sts, err := checkMTASts(domain); err == nil {
        result.MTASTS = sts
    } else {
        result.MTASTS = "error: " + err.Error()
    }

    // DNSSEC – query for DNSKEY records.
    if dnssec, err := checkDNSSEC(domain); err == nil {
        result.DNSSEC = dnssec
    } else {
        result.DNSSEC = "error: " + err.Error()
    }

    // DANE – lookup TLSA records for the SMTP service (port 25).
    if dane, err := checkDANE(domain); err == nil {
        result.DANE = dane
    } else {
        result.DANE = "error: " + err.Error()
    }

    return result, nil
}

// checkMTASts performs a TXT lookup for _mta-sts.<domain> and returns the raw value.
func checkMTASts(domain string) (string, error) {
    txts, err := net.LookupTXT("_mta-sts." + domain)
    if err != nil {
        return "not found", err
    }
    if len(txts) == 0 {
        return "not found", nil
    }
    return txts[0], nil
}

    // checkDNSSEC queries for DNSKEY records; if any are present we consider DNSSEC enabled.
    func checkDNSSEC(domain string) (string, error) {
        client := new(dns.Client)
        msg := new(dns.Msg)
        fqdn := dns.Fqdn(domain)
        msg.SetQuestion(fqdn, dns.TypeDNSKEY)
        resp, _, err := client.Exchange(msg, "127.0.0.1:53")
        if err != nil {
            return "error", err
        }
        if resp.Rcode != dns.RcodeSuccess {
            return "not found", fmt.Errorf("dns response code: %s", dns.RcodeToString[resp.Rcode])
        }
        if len(resp.Answer) == 0 {
            return "not found", nil
        }
        return "enabled", nil
    }

    // checkDANE attempts to retrieve TLSA records for the SMTP service (port 25).
    func checkDANE(domain string) (string, error) {
        client := new(dns.Client)
        msg := new(dns.Msg)
        tlsaName := dns.Fqdn(fmt.Sprintf("_25._tcp.%s", domain))
        msg.SetQuestion(tlsaName, dns.TypeTLSA)
        resp, _, err := client.Exchange(msg, "127.0.0.1:53")
        if err != nil {
            return "error", err
        }
        if resp.Rcode != dns.RcodeSuccess {
            return "not found", fmt.Errorf("dns response code: %s", dns.RcodeToString[resp.Rcode])
        }
        if len(resp.Answer) == 0 {
            return "not found", nil
        }
        var records []string
        for _, a := range resp.Answer {
            if tlsa, ok := a.(*dns.TLSA); ok {
                records = append(records, fmt.Sprintf("%d %d %d %s", tlsa.Usage, tlsa.Selector, tlsa.MatchingType, hex.EncodeToString([]byte(tlsa.Certificate))))
            }
        }
        return strings.Join(records, ", "), nil
    }  // lookupSPF performs a TXT lookup for SPF records and returns the first matching record.
    func lookupSPF(domain string) (string, error) {
        txts, err := net.LookupTXT(domain)
        if err != nil {
            return "error", err
        }
        for _, txt := range txts {
            if strings.HasPrefix(txt, "v=spf1") {
                return txt, nil
            }
        }
        return "not found", nil
    }

    // lookupDKIM performs a TXT lookup for DKIM records for a given selector.
    func lookupDKIM(domain, selector string) (string, error) {
        name := fmt.Sprintf("%s._domainkey.%s", selector, domain)
        txts, err := net.LookupTXT(name)
        if err != nil {
            return "error", err
        }
        if len(txts) == 0 {
            return "not found", nil
        }
        return txts[0], nil
    }

    // lookupDMARC performs a TXT lookup for DMARC records.
    func lookupDMARC(domain string) (string, error) {
        name := fmt.Sprintf("_dmarc.%s", domain)
        txts, err := net.LookupTXT(name)
        if err != nil {
            return "error", err
        }
        if len(txts) == 0 {
            return "not found", nil
        }
        return txts[0], nil
    }
