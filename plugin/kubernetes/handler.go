package kubernetes

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// ServeDNS implements the plugin.Handler interface.
func (k Kubernetes) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true

	zone := plugin.Zones(k.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, r)
	}

	state.Zone = zone

	var (
		records []dns.RR
		extra   []dns.RR
		err     error
	)

	switch state.Type() {
	case "A":
		records, err = plugin.A(&k, zone, state, nil, plugin.Options{})
	case "AAAA":
		records, err = plugin.AAAA(&k, zone, state, nil, plugin.Options{})
	case "TXT":
		records, err = plugin.TXT(&k, zone, state, plugin.Options{})
	case "CNAME":
		records, err = plugin.CNAME(&k, zone, state, plugin.Options{})
	case "PTR":
		records, err = plugin.PTR(&k, zone, state, plugin.Options{})
	case "MX":
		records, extra, err = plugin.MX(&k, zone, state, plugin.Options{})
	case "SRV":
		records, extra, err = plugin.SRV(&k, zone, state, plugin.Options{})
	case "SOA":
		records, err = plugin.SOA(&k, zone, state, plugin.Options{})
	case "NS":
		if state.Name() == zone {
			records, extra, err = plugin.NS(&k, zone, state, plugin.Options{})
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		_, err = plugin.A(&k, zone, state, nil, plugin.Options{})
	}

	if k.IsNameError(err) {
		if k.Fallthrough {
			return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, r)
		}
		return plugin.BackendError(&k, zone, dns.RcodeNameError, state, nil /* err */, plugin.Options{})
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	if len(records) == 0 {
		return plugin.BackendError(&k, zone, dns.RcodeSuccess, state, nil, plugin.Options{})
	}

	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)

	m = dnsutil.Dedup(m)
	state.SizeAndDo(m)
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (k Kubernetes) Name() string { return "kubernetes" }
