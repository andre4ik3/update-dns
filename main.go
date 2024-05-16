package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/cloudflare/cloudflare-go"
	"golang.org/x/net/publicsuffix"
)

func SetupCloudflare(token string) cloudflare.API {
	if token == "" {
		log.Fatalln("Missing -token flag or CLOUDFLARE_API_TOKEN environment variable")
	}

	cf, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		log.Fatalf("Error while creating Cloudflare client: %v\n", err)
	}

	ctx := context.Background()

	resp, err := cf.VerifyAPIToken(ctx)
	if err != nil {
		log.Fatalf("Error while verifying API token: %v\n", err)
	} else if resp.Status != "active" {
		log.Fatalf("Error while verifying API token: token is %s\n", resp.Status)
	}

	return *cf
}

func GetHostAndDomain(hostnameFlag *string, domainFlag *string) (string, string) {
	var hostname = *hostnameFlag
	var domain = *domainFlag

	if hostname == "" {
		realHostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("Failed to get hostname: %v", err)
		} else {
			hostname = realHostname
		}
	}

	if domain == "" {
		realDomain, err := publicsuffix.EffectiveTLDPlusOne(hostname)
		if err != nil {
			log.Fatalf("Failed to get effective TLD: %v", err)
		} else {
			domain = realDomain
		}
	}

	log.Printf(">> Hostname: %s\n", hostname)
	log.Printf(">> Domain: %s\n", domain)

	return hostname, domain
}

func FetchIP(url string) (netip.Addr, error) {
	resp, err := http.Get(url)
	if err != nil {
		return netip.Addr{}, err
	}

	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return netip.Addr{}, err
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(string(bytes)))
	return addr, err
}

func GetIPv4() *netip.Addr {
	addr, err := FetchIP("https://ipv4.icanhazip.com")
	if err != nil {
		return nil
	} else {
		return &addr
	}
}

func GetIPv6() *netip.Addr {
	addr, err := FetchIP("https://ipv6.icanhazip.com")
	if err != nil {
		return nil
	} else {
		return &addr
	}
}

func GetIP() (*netip.Addr, *netip.Addr) {
	log.Println("Fetching current IP address...")

	ipv4 := GetIPv4()
	log.Printf(">> IPv4: %s\n", ipv4)

	ipv6 := GetIPv6()
	log.Printf(">> IPv6: %s\n", ipv6)

	return ipv4, ipv6
}

func GetZoneAndRecords(cf *cloudflare.API, domain string) (*cloudflare.ResourceContainer, []cloudflare.DNSRecord) {
	log.Printf("Fetching zone %s...\n", domain)
	id, err := cf.ZoneIDByName(domain)
	if err != nil {
		log.Fatalf("Failed to get zone for %s: %v\n", domain, err)
	}

	zone := cloudflare.ZoneIdentifier(id)
	records, _, err := cf.ListDNSRecords(context.Background(), zone, cloudflare.ListDNSRecordsParams{})
	if err != nil {
		log.Fatalf("Failed to get records for zone %s: %v\n", zone.Identifier, err)
	}

	log.Printf(">> Zone ID: %s\n", zone.Identifier)

	return zone, records
}

func FindRecord(records *[]cloudflare.DNSRecord, recordType string, hostname string) *cloudflare.DNSRecord {
	for _, record := range *records {
		if record.Name == hostname && record.Type == recordType {
			return &record
		}
	}

	return nil
}

func UpdateRecord(cf *cloudflare.API, zone *cloudflare.ResourceContainer, records *[]cloudflare.DNSRecord, recordType string, hostname string, addr *netip.Addr, proxied *bool) error {
	record := FindRecord(records, recordType, hostname)
	ctx := context.Background()
	var err error

	log.Printf("Updating %s record for %s", recordType, hostname)

	if addr == nil {
		if record == nil {
			log.Println(">> nil -> nil")
		} else {
			log.Printf(">> %s -> nil\n", record.Content)
			err = cf.DeleteDNSRecord(ctx, zone, record.ID)
		}
	} else {
		if record == nil {
			log.Printf(">> nil -> %s", addr)
			_, err = cf.CreateDNSRecord(ctx, zone, cloudflare.CreateDNSRecordParams{
				Name:    hostname,
				Type:    recordType,
				Content: addr.String(),
				Proxied: proxied,
			})
		} else if record.Content != addr.String() {
			log.Printf(">> %s -> %s", record.Content, addr)
			_, err = cf.UpdateDNSRecord(ctx, zone, cloudflare.UpdateDNSRecordParams{
				ID:      record.ID,
				Content: addr.String(),
				Proxied: proxied,
			})
		} else {
			log.Printf(">> %s -> %s", record.Content, addr)
		}
	}

	if err != nil {
		log.Printf(">> Error: %v\n", err)
		return err
	}

	return nil
}

func main() {
	Init()

	// Parse CLI arguments
	hostnameFlag := flag.String("hostname", "", "Hostname to refresh (default: machine hostname)")
	domainFlag := flag.String("domain", "", "Domain to refresh (default: derived from value of -hostname)")
	proxyFlag := flag.Bool("proxy", false, "Whether the records should be proxied (default: false)")
	token := flag.String("token", os.Getenv("CLOUDFLARE_API_TOKEN"), "Cloudflare API token (can also be passed via CLOUDFLARE_API_TOKEN variable)")
	flag.Parse()

	// Setup cloudflare API
	log.Println("Beginning dynamic DNS refresh")
	cf := SetupCloudflare(*token)

	// We will use the machine hostname to determine what zone and record to update
	hostname, domain := GetHostAndDomain(hostnameFlag, domainFlag)

	// Get our IP address(es)
	ipv4, ipv6 := GetIP()

	// Get zone for our domain
	zone, records := GetZoneAndRecords(&cf, domain)

	// Update records
	v4err := UpdateRecord(&cf, zone, &records, "A", hostname, ipv4, proxyFlag)
	v6err := UpdateRecord(&cf, zone, &records, "AAAA", hostname, ipv6, proxyFlag)

	// If at least one of the results failed, exit with an error code
	if v4err != nil {
		log.Fatalf("Failed to update IPv4 DNS record: %v\n", v4err)
	} else if v6err != nil {
		log.Fatalf("Failed to update IPv6 DNS record: %v\n", v6err)
	} else {
		log.Println("Finished dynamic DNS refresh")
	}
}
