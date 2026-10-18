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
	welcome := appendHello(nil, tagWelcome, self)
	welcome = append(welcome, proof(cookie, "accept", h.challenge)...)
	if err := writePacket(conn, welcome); err != nil {
		return info, err
	}
	b, err = readPacket(conn)
	if err != nil {
		return info, err
	}
	if len(b) == 0 || b[0] != tagProof || !hmac.Equal(b[1:], proof(cookie, "dial", self.challenge)) {
		return info, errBadCookie
	}
	return info, nil
}
