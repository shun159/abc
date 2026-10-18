package dist

// acceptHandshake runs the handshake of the acceptor on conn. decide is
// told who dials, and returns why to refuse, if it does.
func acceptHandshake(conn net.Conn, self hello, cookie string, decide func(peerInfo) (refuse string)) (peerInfo, error) {
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	defer conn.SetDeadline(time.Time{})
	b, err := readPacket(conn)
	if err != nil {
		return peerInfo{}, err
	}
	h, _, err := parseHello(b, tagHello)
	if err != nil {
		return peerInfo{}, err
	}
	info := peerInfo{h.name, h.creation}
	if why := decide(info); why != "" {
		writePacket(conn, append([]byte{tagRefuse}, why...))
		return info, fmt.Errorf("%w: %s", ErrRefused, why)
	}
	if err := welcome(conn, self, cookie, h.challenge); err != nil {
		return info, err
	}
	return info, readProof(conn, cookie, self.challenge)
}

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
	proved := len(b) > 0 && b[0] == tagProof && hmac.Equal(b[1:], proof(cookie, "dial", challenge))
	if !proved {
		return errBadCookie
	}
	return nil
}
