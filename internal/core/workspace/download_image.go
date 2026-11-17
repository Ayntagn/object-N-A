package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var (
	errInvalidImageURL = errors.New("expected an HTTP or HTTPS image URL")
	errPrivateImageURL = errors.New("image host must use a public address")
	errImageRedirects  = errors.New("too many image redirects")
	imageClient        = newImageDownloadClient()
)

func newImageDownloadClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:            dialPublicImageHost,
			ForceAttemptHTTP2:      true,
			MaxIdleConns:           16,
			IdleConnTimeout:        30 * time.Second,
			MaxResponseHeaderBytes: 64 << 10,
		},
		Timeout: 20 * time.Second,
		CheckRedirect: func(next *http.Request, previous []*http.Request) error {
			if len(previous) >= 5 {
				return errImageRedirects
			}
			next.Header.Del("Referer")
			return validateImageURL(next.URL)
		},
	}
}

// DownloadImage fetches a raster image explicitly dropped by the user.
func DownloadImage(ctx context.Context, address string) ([]byte, error) {
	return downloadImage(ctx, imageClient, address)
}

func downloadImage(ctx context.Context, client *http.Client, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, errInvalidImageURL
	}
	if err := validateImageURL(req.URL); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/webp")
	response, err := client.Do(req)
	if err != nil {
		// The error's URL can contain a signed query.
		for _, known := range []error{context.Canceled, context.DeadlineExceeded, errInvalidImageURL, errPrivateImageURL, errImageRedirects} {
			if errors.Is(err, known) {
				return nil, known
			}
		}
		return nil, errors.New("could not download the image")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("image download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxImageBytes {
		return nil, fmt.Errorf("downloaded image exceeds the %d MiB image limit", maxImageBytes>>20)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("could not read the downloaded image")
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("downloaded image exceeds the %d MiB image limit", maxImageBytes>>20)
	}
	if _, err := checkImageData("downloaded image", data); err != nil {
		return nil, err
	}
	return data, nil
}

func validateImageURL(endpoint *url.URL) error {
	if endpoint.User != nil || endpoint.Hostname() == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return errInvalidImageURL
	}
	host := strings.ToLower(strings.TrimSuffix(endpoint.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errPrivateImageURL
	}
	if ip, err := netip.ParseAddr(host); err == nil && !publicImageIP(ip) {
		return errPrivateImageURL
	}
	return nil
}

func dialPublicImageHost(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	dialErr := errPrivateImageURL
	for _, ip := range addresses {
		if !imageDialAddressAllowed(host, ip) {
			continue
		}
		// Dial the checked IP directly. A second DNS lookup could return a
		// different address. The HTTP transport still verifies TLS for host.
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		dialErr = err
	}
	return nil, dialErr
}

// Some system DNS proxies assign addresses from this range to public
// hostnames and route the connection to the hostname. Keep literal URLs
// subject to the regular public-address checks.
func imageDialAddressAllowed(host string, ip netip.Addr) bool {
	if publicImageIP(ip) {
		return true
	}
	if ip.Zone() != "" || !imageFakeIPNetwork.Contains(ip.Unmap()) {
		return false
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	dot := strings.LastIndexByte(host, '.')
	if dot <= 0 || dot == len(host)-1 || strings.HasSuffix("."+host, ".home.arpa") {
		return false
	}
	// A final label starting with a letter also excludes numeric address
	// spellings accepted by some system resolvers.
	switch tld := host[dot+1:]; tld {
	case "local", "localhost", "localdomain", "internal", "lan":
		return false
	default:
		return 'a' <= tld[0] && tld[0] <= 'z'
	}
}

var (
	imageIPv6Internet  = netip.MustParsePrefix("2000::/3")
	imageFakeIPNetwork = netip.MustParsePrefix("198.18.0.0/15")
)

var reservedImageNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	imageFakeIPNetwork,
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func publicImageIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.Zone() != "" || !ip.IsGlobalUnicast() || ip.IsPrivate() || (ip.Is6() && !imageIPv6Internet.Contains(ip)) {
		return false
	}
	for _, reserved := range reservedImageNetworks {
		if reserved.Contains(ip) {
			return false
		}
	}
	return true
}
