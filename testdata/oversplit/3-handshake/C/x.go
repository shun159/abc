package dist

// acceptHandshake runs the handshake of the acceptor on conn. decide is
// told who dials, and returns why to refuse, if it does.
func acceptHandshake(conn net.Conn, self hello, cookie string, decide func(peerInfo) (refuse string)) (peerInfo, error) {
	defer setDeadline(conn)()
	h, err := readHelloOf(conn)
	if err != nil {
		return peerInfo{}, err
	}
	info := infoOf(h)
	if err := refuseIf(conn, decide, info); err != nil {
		return info, err
	}
	if err := welcome(conn, self, cookie, h.challenge); err != nil {
		return info, err
	}
	return info, readProof(conn, cookie, self.challenge)
}

// setDeadline sets the deadline of the handshake on conn, and returns
// what clears it.
func setDeadline(conn net.Conn) func() {
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	return func() { conn.SetDeadline(time.Time{}) }
}

// readHelloOf reads the hello of the dialer.
func readHelloOf(conn net.Conn) (hello, error) {
	b, err := readPacket(conn)
	if err != nil {
		return hello{}, err
	}
	return parseHelloPacket(b)
}

// parseHelloPacket parses a hello.
func parseHelloPacket(b []byte) (hello, error) {
	h, _, err := parseHello(b, tagHello)
	return h, err
}

// infoOf is who h is.
func infoOf(h hello) peerInfo { return peerInfo{h.name, h.creation} }

// refuseIf refuses the dialer, if decide does.
func refuseIf(conn net.Conn, decide func(peerInfo) string, info peerInfo) error {
	why := decide(info)
	if why == "" {
		return nil
	}
	return refuse(conn, why)
}

// refuse refuses the dialer, telling why.
func refuse(conn net.Conn, why string) error {
	writePacket(conn, refusal(why))
	return fmt.Errorf("%w: %s", ErrRefused, why)
}

// refusal is the packet of a refusal.
func refusal(why string) []byte { return append([]byte{tagRefuse}, why...) }

// welcome welcomes the dialer on conn, with the proof of the cookie for
// its challenge.
func welcome(conn net.Conn, self hello, cookie string, challenge []byte) error {
	w := appendHello(nil, tagWelcome, self)
	w = append(w, proof(cookie, "accept", challenge)...)
	return writePacket(conn, w)
}

// readProof reads the dialer's proof of the cookie, for our challenge.
func readProof(conn net.Conn, cookie string, challenge []byte) error {
	b, err := readPacket(conn)
	if err != nil {
		return err
	}
	return checkProof(b, cookie, challenge)
}

// checkProof checks the proof b.
func checkProof(b []byte, cookie string, challenge []byte) error {
	if !isProof(b, cookie, challenge) {
		return errBadCookie
	}
	return nil
}

// isProof tells whether b is the proof of the cookie for the challenge.
func isProof(b []byte, cookie string, challenge []byte) bool {
	return len(b) > 0 && b[0] == tagProof && hmac.Equal(b[1:], proof(cookie, "dial", challenge))
}
