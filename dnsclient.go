package ipisp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const hexDigit = "0123456789abcdef"

type dnsClient struct {
}

func NewDnsClient() (client *dnsClient, err error) {
	return &dnsClient{}, nil
}

func (c *dnsClient) LookupIPs(ips []net.IP) ([]Response, error) {
	ret := make([]Response, len(ips))

	for _, ip := range ips {
		resp, err := c.LookupIP(ip)
		if err != nil {
			return ret, err
		}
		ret = append(ret, *resp)
	}
	return ret, nil
}

func (c *dnsClient) LookupIP(ip net.IP) (*Response, error) {
	lookupName, err := c.getLookupName(ip)
	txts, err := net.LookupTXT(lookupName)
	if err != nil {
		return nil, err
	}

	for _, txt := range txts {
		values := strings.Split(txt, "|")
		if len(values) != 5 {
			return nil, fmt.Errorf("Received unrecognized response: %s", txt)
		}
		for k := range values {
			values[k] = strings.TrimSpace(values[k])
		}

		ret := &Response{
			IP:       ip,
			Registry: values[3],
		}

		var err error
		asn, err := strconv.Atoi(values[0])
		if err != nil {
			return nil, fmt.Errorf("Could not parse ASN (%s): %s", values[0], err)
		}
		ret.ASN = ASN(asn)

		ret.Country, err = NewCountryFromCode(values[2])
		if err != nil {
			return nil, fmt.Errorf("Could not parse CountryCode (%s): %s", values[2], err)
		}

		_, ret.Range, err = net.ParseCIDR(values[1])
		if err != nil {
			return nil, fmt.Errorf("Could not parse Range (%s): %s", values[1], err)
		}

		if values[4] != "" { // There's not always an allocation date available :(
			ret.Allocated, err = time.Parse("2006-01-02", values[4])
			if err != nil {
				return nil, fmt.Errorf("Could not parse date (%s): %s", values[4], err)
			}
		}

		asnResponse, err := c.LookupASN(ret.ASN)
		if err != nil {
			return nil, fmt.Errorf("Could not retrieve ASN (%s): %s", ret.ASN.String(), err.Error())
		}

		ret.Name = asnResponse.Name

		return ret, nil

	}

	return nil, fmt.Errorf("No records found")
}

func (c *dnsClient) LookupASNs(asns []ASN) ([]Response, error) {
	ret := make([]Response, len(asns))

	for _, asn := range asns {
		resp, err := c.LookupASN(asn)
		if err != nil {
			return ret, err
		}
		ret = append(ret, *resp)
	}
	return ret, nil
}

func (c *dnsClient) LookupASN(asn ASN) (*Response, error) {
	txts, err := net.LookupTXT(asn.String() + ".asn.cymru.com")
	if err != nil {
		return nil, err
	}

	for _, txt := range txts {
		values := strings.Split(txt, "|")
		if len(values) != 5 {
			return nil, fmt.Errorf("Received unrecognized response in AS lookup: %s", txt)
		}
		for k := range values {
			values[k] = strings.TrimSpace(values[k])
		}

		ret := &Response{
			ASN:      asn,
			Registry: values[2],
			Name:     NewName(values[4]),
		}

		ret.Country, err = NewCountryFromCode(values[1])
		if err != nil {
			return nil, fmt.Errorf("Could not parse CountryCode (%s): %s", values[1], err)
		}

		if values[3] != "" {
			ret.Allocated, err = time.Parse("2006-01-02", values[3])
			if err != nil {
				return nil, fmt.Errorf("Could not parse date (%s): %s", values[3], err)
			}
		}

		return ret, nil
	}

	return nil, fmt.Errorf("No records found")
}

func (c *dnsClient) Close() error {
	return nil
}

func (c *dnsClient) getLookupName(ip net.IP) (string, error) {
	if p4 := ip.To4(); len(p4) == net.IPv4len {
		return fmt.Sprintf("%d.%d.%d.%d.origin.asn.cymru.com", ip[15], ip[14], ip[13], ip[12]), nil
	}

	if len(ip) != net.IPv6len {
		return "", fmt.Errorf("Could not parse IP. Invalid length (%d)", len(ip))
	}

	sep := []byte(`.`)[0]
	b := make([]byte, 0, 64)
	for i := 16; i >= 2; i -= 2 {
		for j := 0; j <= 3; j++ {
			v := ((uint32(ip[i-2]) << 8) | uint32(ip[i-1])) >> uint(j*4)
			b = append(b, hexDigit[v&0xf], sep)
		}
	}

	return fmt.Sprintf("%s.origin6.asn.cymru.com", (b[:63])), nil
}
