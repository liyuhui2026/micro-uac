package sip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/emiago/sipgo"
	siplib "github.com/emiago/sipgo/sip"
	"github.com/google/uuid"
	"github.com/liyuhui/micro-uac/internal/config"
	"github.com/liyuhui/micro-uac/internal/domain"
	"github.com/liyuhui/micro-uac/internal/sdp"
	"github.com/rs/zerolog"
)

type Client struct {
	cfg        config.SIPConfig
	logger     zerolog.Logger
	ua         *sipgo.UserAgent
	client     *sipgo.Client
	server     *sipgo.Server
	dialogs    *sipgo.DialogClientCache
	packetConn net.PacketConn
}

func NewClient(cfg config.SIPConfig, logger zerolog.Logger) (*Client, func() error, error) {
	ua, err := sipgo.NewUA(sipgo.WithUserAgent(cfg.UserAgent))
	if err != nil {
		return nil, nil, fmt.Errorf("create sip ua: %w", err)
	}

	bindHost, port, err := splitBindHostPort(cfg.ListenAddr)
	if err != nil {
		return nil, nil, err
	}
	advertisedHost, err := advertisedHost(cfg)
	if err != nil {
		return nil, nil, err
	}

	server, err := sipgo.NewServer(ua)
	if err != nil {
		ua.Close()
		return nil, nil, fmt.Errorf("create sip server: %w", err)
	}

	packetConn, err := net.ListenPacket("udp", net.JoinHostPort(bindHost, fmt.Sprintf("%d", port)))
	if err != nil {
		ua.Close()
		return nil, nil, fmt.Errorf("listen sip udp: %w", err)
	}
	localAddr := packetConn.LocalAddr().String()

	client, err := sipgo.NewClient(
		ua,
		sipgo.WithClientHostname(advertisedHost),
		sipgo.WithClientPort(port),
		sipgo.WithClientConnectionAddr(localAddr),
	)
	if err != nil {
		_ = packetConn.Close()
		ua.Close()
		return nil, nil, fmt.Errorf("create sip client: %w", err)
	}

	contactHDR := siplib.ContactHeader{
		Address: siplib.Uri{
			Scheme: "sip",
			Host:   advertisedHost,
			Port:   port,
		},
	}
	dialogs := sipgo.NewDialogClientCache(client, contactHDR)

	c := &Client{
		cfg:        cfg,
		logger:     logger,
		ua:         ua,
		client:     client,
		server:     server,
		dialogs:    dialogs,
		packetConn: packetConn,
	}

	server.OnBye(func(req *siplib.Request, tx siplib.ServerTransaction) {
		c.logRequest("inbound BYE", req)
		if err := dialogs.ReadBye(req, tx); err != nil {
			logger.Error().Err(err).Msg("handle inbound BYE")
		}
	})

	go func() {
		if err := server.ServeUDP(packetConn); err != nil {
			c.logger.Error().Err(err).Msg("sip udp server stopped")
		}
	}()

	if err := waitForSIPListener(ua, localAddr); err != nil {
		_ = packetConn.Close()
		_ = ua.Close()
		return nil, nil, err
	}

	cleanup := func() error {
		var errs []error
		if err := packetConn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close sip packet conn: %w", err))
		}
		if err := ua.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close sip ua: %w", err))
		}
		return errors.Join(errs...)
	}

	return c, cleanup, nil
}

func (c *Client) Dial(ctx context.Context, req domain.CallRequest, offer sdp.Offer) (domain.EstablishedCall, error) {
	var requestURI siplib.Uri
	if err := siplib.ParseUri(req.RequestURI, &requestURI); err != nil {
		return nil, fmt.Errorf("parse request uri: %w", err)
	}

	invite := siplib.NewRequest(siplib.INVITE, requestURI)
	invite.SetTransport("UDP")
	invite.SetDestination(req.FSAddr)
	invite.AppendHeader(siplib.NewHeader("From", withTag(req.From, uuid.NewString())))
	toHeader, err := rewriteTargetHost(req.To, req.FSAddr)
	if err != nil {
		return nil, err
	}
	targetURI, err := buildTargetURI(req.TargetURI, req.LineAddr)
	if err != nil {
		return nil, err
	}
	invite.AppendHeader(siplib.NewHeader("To", toHeader))
	invite.AppendHeader(siplib.NewHeader("X-Sip-Client-Target-Uri", targetURI))
	invite.AppendHeader(siplib.NewHeader("Content-Type", "application/sdp"))
	invite.SetBody([]byte(offer.Body))
	c.logRequest("outbound INVITE", invite)

	session, err := c.dialogs.WriteInvite(ctx, invite)
	if err != nil {
		return nil, fmt.Errorf("send invite: %w", err)
	}
	// Some PBXes advertise an unreachable Contact while responding from a reachable
	// address. Reuse the response source for in-dialog requests like ACK/BYE.
	session.UA.RewriteContact = true
	if err := session.WaitAnswer(ctx, sipgo.AnswerOptions{
		OnResponse: func(res *siplib.Response) error {
			c.logResponse("inbound SIP response", res)
			return nil
		},
	}); err != nil {
		return nil, fmt.Errorf("wait invite answer: %w", err)
	}

	answer, err := sdp.ParseAnswer(string(session.InviteResponse.Body()), req.Codec)
	if err != nil {
		return nil, err
	}

	c.logger.Info().
		Str("remote_host", answer.Host).
		Int("remote_port", answer.Port).
		Uint8("payload_type", answer.PayloadType).
		Str("codec", string(answer.Codec)).
		Msg("parsed remote media from SDP answer")

	if err := session.Ack(ctx); err != nil {
		return nil, fmt.Errorf("send ack: %w", err)
	}
	contactValue := ""
	if contact := session.InviteResponse.Contact(); contact != nil {
		contactValue = contact.Value()
	}
	c.logger.Info().
		Str("response_source", session.InviteResponse.Source()).
		Str("response_contact", contactValue).
		Msg("outbound ACK sent")

	callIDHeader := session.InviteResponse.CallID()
	sipCallID := ""
	if callIDHeader != nil {
		sipCallID = callIDHeader.Value()
	}
	return &dialog{
		callID:    uuid.NewString(),
		sipCallID: sipCallID,
		remote:    answer,
		session:   session,
		logger:    c.logger,
	}, nil
}

type dialog struct {
	callID    string
	sipCallID string
	remote    domain.RemoteMedia
	session   *sipgo.DialogClientSession
	logger    zerolog.Logger
	closed    bool
}

func (d *dialog) CallID() string {
	return d.callID
}

func (d *dialog) SIPCallID() string {
	return d.sipCallID
}

func (d *dialog) RemoteMedia() domain.RemoteMedia {
	return d.remote
}

func (d *dialog) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.session.Context().Done():
		return context.Cause(d.session.Context())
	}
}

func (d *dialog) Hangup(ctx context.Context) error {
	if d.closed {
		return nil
	}
	if d.session.LoadState() == siplib.DialogStateEnded {
		d.closed = true
		return nil
	}
	byeReq := buildDialogByeForLog(d.session)
	d.logRequest("outbound BYE", byeReq)

	tx, err := d.session.TransactionRequest(ctx, byeReq)
	if err != nil {
		return fmt.Errorf("send bye: %w", err)
	}
	defer d.session.Close()
	defer tx.Terminate()

	select {
	case res := <-tx.Responses():
		if res == nil {
			return fmt.Errorf("send bye: empty response")
		}
		d.logResponse("inbound response for BYE", res)
		if res.StatusCode != 200 {
			return fmt.Errorf("send bye: unexpected response %d %s", res.StatusCode, res.Reason)
		}
		d.session.InviteResponse = res
	case <-tx.Done():
		return fmt.Errorf("send bye: %w", tx.Err())
	case <-ctx.Done():
		return fmt.Errorf("send bye: %w", ctx.Err())
	}

	d.closed = true
	return nil
}

func splitBindHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("parse sip.listen_addr: %w", err)
	}
	port, err := net.LookupPort("udp", portStr)
	if err != nil {
		return "", 0, fmt.Errorf("parse sip.listen_addr port: %w", err)
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return host, port, nil
}

func rewriteTargetHost(toValue, fsAddr string) (string, error) {
	host, port, err := net.SplitHostPort(fsAddr)
	if err != nil {
		return "", fmt.Errorf("parse fs_addr: %w", err)
	}
	start := strings.IndexByte(toValue, ':')
	at := strings.LastIndex(toValue, "@")
	if start == -1 || at == -1 || at <= start {
		return "", fmt.Errorf("parse to header: unsupported value %q", toValue)
	}
	suffixIdx := strings.IndexAny(toValue[at:], ">;")
	if suffixIdx == -1 {
		return toValue[:at+1] + net.JoinHostPort(host, port), nil
	}
	suffixIdx += at
	return toValue[:at+1] + net.JoinHostPort(host, port) + toValue[suffixIdx:], nil
}

func buildTargetURI(targetURI, lineAddr string) (string, error) {
	var uri siplib.Uri
	if err := siplib.ParseUri(targetURI, &uri); err != nil {
		return "", fmt.Errorf("parse target_uri: %w", err)
	}
	if uri.User == "" {
		return "", fmt.Errorf("parse target_uri: missing destination number in %q", targetURI)
	}
	return "sip:" + uri.User + "@" + lineAddr, nil
}

func withTag(value, tag string) string {
	if strings.Contains(strings.ToLower(value), ";tag=") {
		return value
	}
	return fmt.Sprintf("%s;tag=%s", value, tag)
}

func advertisedHost(cfg config.SIPConfig) (string, error) {
	if cfg.ExternalIP != "" {
		return cfg.ExternalIP, nil
	}

	host, _, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil {
		return "", fmt.Errorf("parse sip.listen_addr: %w", err)
	}
	if host == "" {
		return "", fmt.Errorf("sip.external_ip is required when sip.listen_addr host is empty")
	}
	if ip, err := netip.ParseAddr(host); err == nil && ip.IsUnspecified() {
		return "", fmt.Errorf("sip.external_ip is required when sip.listen_addr uses unspecified host %q", host)
	}
	return host, nil
}

func waitForSIPListener(ua *sipgo.UserAgent, localAddr string) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := ua.TransportLayer().GetConnection("udp", localAddr); err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("sip listener not ready for %s", localAddr)
}

func (c *Client) logRequest(message string, req *siplib.Request) {
	if req == nil {
		return
	}
	c.logger.Info().
		Str("sip_message", message).
		Str("method", string(req.Method)).
		Str("transport", req.Transport()).
		Str("destination", req.Destination()).
		Str("body", req.String()).
		Msg("sip request")
}

func (c *Client) logResponse(message string, res *siplib.Response) {
	if res == nil {
		return
	}
	c.logger.Info().
		Str("sip_message", message).
		Int("status_code", int(res.StatusCode)).
		Str("reason", res.Reason).
		Str("transport", res.Transport()).
		Str("source", res.Source()).
		Str("body", res.String()).
		Msg("sip response")
}

func buildDialogByeForLog(session *sipgo.DialogClientSession) *siplib.Request {
	if session == nil || session.InviteRequest == nil || session.InviteResponse == nil {
		return nil
	}

	recipient := &session.InviteRequest.Recipient
	if cont := session.InviteResponse.Contact(); cont != nil {
		recipient = &cont.Address
	}

	byeReq := siplib.NewRequest(siplib.BYE, *recipient.Clone())
	if len(session.InviteRequest.GetHeaders("Route")) > 0 {
		siplib.CopyHeaders("Route", session.InviteRequest, byeReq)
	}
	maxForwardsHeader := siplib.MaxForwardsHeader(70)
	byeReq.AppendHeader(&maxForwardsHeader)
	if h := session.InviteRequest.From(); h != nil {
		byeReq.AppendHeader(siplib.HeaderClone(h))
	}
	if h := session.InviteResponse.To(); h != nil {
		byeReq.AppendHeader(siplib.HeaderClone(h))
	}
	if h := session.InviteRequest.CallID(); h != nil {
		byeReq.AppendHeader(siplib.HeaderClone(h))
	}
	if h := session.InviteRequest.CSeq(); h != nil {
		byeReq.AppendHeader(siplib.HeaderClone(h))
	}
	cseq := byeReq.CSeq()
	cseq.MethodName = siplib.BYE
	cseq.SeqNo = session.CSEQ() + 1
	byeReq.SetBody(nil)
	byeReq.SetTransport(session.InviteRequest.Transport())
	byeReq.SetSource(session.InviteRequest.Source())
	return byeReq
}

func (d *dialog) logRequest(message string, req *siplib.Request) {
	if req == nil {
		return
	}
	d.logger.Info().
		Str("sip_message", message).
		Str("method", string(req.Method)).
		Str("transport", req.Transport()).
		Str("destination", req.Destination()).
		Str("body", req.String()).
		Msg("sip request")
}

func (d *dialog) logResponse(message string, res *siplib.Response) {
	if res == nil {
		return
	}
	d.logger.Info().
		Str("sip_message", message).
		Int("status_code", int(res.StatusCode)).
		Str("reason", res.Reason).
		Str("transport", res.Transport()).
		Str("source", res.Source()).
		Str("body", res.String()).
		Msg("sip response")
}
