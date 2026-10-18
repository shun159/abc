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
			if m.From != top {
				continue
			}
			if stoppers != nil {
				for _, from := range stoppers {
					actor.SendReply(self, from, nil)
				}
				return nil
			}
			r.ended(a, m.Reason)
			return nil
		case actor.CallMsg:
			if _, ok := m.Req.(stopApp); !ok {
				continue
			}
			if stoppers == nil {
				self.Exit(top, proc.Shutdown)
				if d := a.app.Shutdown; d > 0 {
					n, pid := self.Node(), self.PID()
					time.AfterFunc(d, func() { n.Send(pid, killTop{}) })
				}
			}
			stoppers = append(stoppers, m.From)
		case killTop:
			self.Exit(top, proc.Kill)
		}
	}
}
