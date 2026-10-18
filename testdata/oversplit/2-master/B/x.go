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
		switch m := msg.(type) {
		case proc.ExitMsg:
			if m.From == top {
				r.topEnded(self, a, m.Reason, stoppers)
				return nil
			}
		case actor.CallMsg:
			if _, ok := m.Req.(stopApp); ok {
				stoppers = stopTop(self, a, top, stoppers, m.From)
			}
		case killTop:
			self.Exit(top, proc.Kill)
		}
	}
}

// topEnded is the end of the top of a: the callers that stopped it, if
// any, are replied to; or else it ended by itself.
func (r *Running) topEnded(self *proc.Self, a *running, reason error, stoppers []actor.From) {
	if stoppers == nil {
		r.ended(a, reason)
		return
	}
	for _, from := range stoppers {
		actor.SendReply(self, from, nil)
	}
}

// stopTop stops the top of a for the caller from: at the first caller, a
// shutdown, and a kill once the shutdown time of a is over, if it has
// one. It returns the callers to reply to when the top has ended.
func stopTop(self *proc.Self, a *running, top proc.PID, stoppers []actor.From, from actor.From) []actor.From {
	if stoppers == nil {
		self.Exit(top, proc.Shutdown)
		if d := a.app.Shutdown; d > 0 {
			n, pid := self.Node(), self.PID()
			time.AfterFunc(d, func() { n.Send(pid, killTop{}) })
		}
	}
	return append(stoppers, from)
}
