package application

// master watches the top of an application until it ends, or stops it
// when asked.
func (r *Running) master(self *proc.Self, a *running, top proc.PID) error {
	var stoppers []actor.From
	for {
		msg, err := self.Receive(context.Background())
		if err != nil {
			return err
		}
		var done bool
		if stoppers, done = r.handle(self, a, top, stoppers, msg); done {
			return nil
		}
	}
}

// handle handles msg, and returns the callers to reply to, and whether
// the master is done.
func (r *Running) handle(self *proc.Self, a *running, top proc.PID, stoppers []actor.From, msg any) ([]actor.From, bool) {
	switch m := msg.(type) {
	case proc.ExitMsg:
		return stoppers, r.exited(self, a, top, stoppers, m)
	case actor.CallMsg:
		return r.called(self, a, top, stoppers, m), false
	case killTop:
		r.kill(self, top, true)
	}
	return stoppers, false
}

// exited handles an exit, and reports whether it was of the top.
func (r *Running) exited(self *proc.Self, a *running, top proc.PID, stoppers []actor.From, m proc.ExitMsg) bool {
	if m.From != top {
		return false
	}
	r.end(self, a, stoppers, m.Reason, stoppers != nil)
	return true
}

// end ends the master, stopped or not.
func (r *Running) end(self *proc.Self, a *running, stoppers []actor.From, reason error, stopped bool) {
	if stopped {
		replyAll(self, stoppers)
		return
	}
	r.ended(a, reason)
}

// replyAll replies to the callers.
func replyAll(self *proc.Self, froms []actor.From) {
	for _, from := range froms {
		actor.SendReply(self, from, nil)
	}
}

// called handles a call, and returns the callers to reply to.
func (r *Running) called(self *proc.Self, a *running, top proc.PID, stoppers []actor.From, m actor.CallMsg) []actor.From {
	if _, ok := m.Req.(stopApp); !ok {
		return stoppers
	}
	if stoppers == nil {
		r.kill(self, top, false)
		r.killLater(self, a)
	}
	return append(stoppers, m.From)
}

// kill stops the top, hard or not.
func (r *Running) kill(self *proc.Self, top proc.PID, hard bool) {
	if hard {
		self.Exit(top, proc.Kill)
		return
	}
	self.Exit(top, proc.Shutdown)
}

// killLater kills the top once the shutdown time of a is over.
func (r *Running) killLater(self *proc.Self, a *running) {
	if d := a.app.Shutdown; d > 0 {
		n, pid := self.Node(), self.PID()
		time.AfterFunc(d, func() { n.Send(pid, killTop{}) })
	}
}
