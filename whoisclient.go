//Package ipisp provides a wrapper to team-cymru.com IP to ASN service.
//ipisp uses Cymru's netcat interface
package ipisp

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"
)

var ncEOL = []byte("\r\n")

//Timeout is the TCP connection timeout
var Timeout = time.Second * 10

//Common errors
var (
	ErrUnexpectedTokens = errors.New("Unexpected tokens while reading Cymru response.")
)

const (
	netcatIPTokensLength  = 7
	netcatASNTokensLength = 5
)

//Network addresses
const (
	cymruNetcatAddress = "whois.cymru.com:43"
)

//Client wraps the team-cyru services
type whoisClient struct {
	conn net.Conn
	w    *bufio.Writer
	sc   *bufio.Scanner
	ncmu *sync.Mutex
}

//NewClient returns a pointer to a new connected IPISP client
func NewWhoisClient() (client *whoisClient, err error) {
	client = &whoisClient{}
	client.conn, err = net.DialTimeout("tcp", cymruNetcatAddress, Timeout)
	client.ncmu = &sync.Mutex{}
	if err != nil {
		return
	}
	client.w = bufio.NewWriter(client.conn)
	client.sc = bufio.NewScanner(client.conn)

	client.w.Write([]byte("begin"))
	client.w.Write(ncEOL)
	client.w.Write([]byte("verbose"))
	client.w.Write(ncEOL)

	err = client.w.Flush()
	if err != nil {
		return
	}

	//Discard first hello line
	client.sc.Scan()
	client.sc.Bytes()
	err = client.sc.Err()
	return
}

//Close closes a client.
func (c *whoisClient) Close() error {
	c.w.Write([]byte("end"))
	c.w.Write(ncEOL)
	return c.conn.Close()
}

//LookupIPs looks up IPs and returns a slice of responses the same size as the input slice of IPs
//The response slice will be in the same order as the input IPs
func (c *whoisClient) LookupIPs(ips []net.IP) (resp []Response, err error) {
	resp = make([]Response, 0, len(ips))

	c.ncmu.Lock()
	defer c.ncmu.Unlock()
	for _, ip := range ips {
		c.w.WriteString(ip.String())
		c.w.Write(ncEOL)
		if err = c.w.Flush(); err != nil {
			return resp, err
		}
	}
	//Raw response
	var raw []byte
	var tokens [][]byte
	var asn int

	var finished bool

	//Read results
	for !finished && c.sc.Scan() {

		raw = c.sc.Bytes()
		if bytes.HasPrefix(raw, []byte("Error: ")) {
			return resp, errors.New(string(bytes.TrimSpace(bytes.TrimLeft(raw, "Error: "))))
		}
		tokens = bytes.Split(raw, []byte{'|'})

		if len(tokens) != netcatIPTokensLength {
			return resp, ErrUnexpectedTokens
		}

		//Trim excess whitespace from tokens
		for i := range tokens {
			tokens[i] = bytes.TrimSpace(tokens[i])
		}

		re := Response{}

		//Read ASN
		if asn, err = strconv.Atoi(string(tokens[0])); err != nil {
			return
		}
		re.ASN = ASN(asn)

		//Read IP
		re.IP = net.ParseIP(string(tokens[1]))

		//Read range
		if _, re.Range, err = net.ParseCIDR(string(tokens[2])); err != nil {
			return
		}

		//Read country
		re.Country, _ = NewCountryFromCode(string(tokens[3]))

		//Read registry
		re.Registry = string(tokens[4])

		//Read allocated. Ignore error as a lot of entries don't have an allocated value.
		re.Allocated, _ = time.Parse("2006-01-02", string(tokens[5]))

		//Read name
		re.Name = NewName(string(tokens[6]))

		//Add to response slice
		resp = append(resp, re)
		if len(resp) == cap(resp) {
			finished = true
		}
	}
	return resp, err
}

//LookupIP is a single IP convenience proxy of LookupIPs
func (c *whoisClient) LookupIP(ip net.IP) (*Response, error) {
	resp, err := c.LookupIPs([]net.IP{ip})
	if len(resp) == 0 {
		return nil, err
	}
	return &resp[0], err
}

//LookupASNs looks up ASNs. Response IP and Range fields are zeroed
func (c *whoisClient) LookupASNs(asns []ASN) (resp []Response, err error) {
	resp = make([]Response, 0, len(asns))

	c.ncmu.Lock()
	defer c.ncmu.Unlock()
	for _, asn := range asns {
		c.w.WriteString(asn.String())
		c.w.Write(ncEOL)
		if err = c.w.Flush(); err != nil {
			return resp, err
		}
	}

	//Raw response
	var raw []byte
	var tokens [][]byte
	var asn int

	var finished bool

	//Read results
	for !finished && c.sc.Scan() {
		raw = c.sc.Bytes()
		if bytes.HasPrefix(raw, []byte("Error: ")) {
			return resp, errors.New(string(bytes.TrimSpace(bytes.TrimLeft(raw, "Error: "))))
		}
		tokens = bytes.Split(raw, []byte{'|'})

		if len(tokens) != netcatASNTokensLength {
			return resp, ErrUnexpectedTokens
		}

		//Trim excess whitespace from tokens
		for i := range tokens {
			tokens[i] = bytes.TrimSpace(tokens[i])
		}

		re := Response{}

		//Read ASN
		if asn, err = strconv.Atoi(string(tokens[0])); err != nil {
			return
		}
		re.ASN = ASN(asn)

		//Read country
		re.Country, _ = NewCountryFromCode(string(tokens[1]))

		//Read registry
		re.Registry = string(tokens[2])

		//Read allocated. Ignore error as a lot of entries don't have an allocated value.
		re.Allocated, _ = time.Parse("2006-01-02", string(tokens[3]))

		//Read name
		re.Name = NewName(string(tokens[4]))

		//Add to response slice
		resp = append(resp, re)
		if len(resp) == cap(resp) {
			finished = true
		}
	}
	return resp, err
}

//LookupASN is a single ASN convenience proxy of LookupASNs
func (c *whoisClient) LookupASN(asn ASN) (*Response, error) {
	resp, err := c.LookupASNs([]ASN{asn})
	return &resp[0], err
}
