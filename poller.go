package telebot

import (
	"time"
)

// Poller is a provider of Updates.
//
// All pollers must implement Poll(), which accepts bot
// pointer and subscription channel and start polling
// synchronously straight away.
type Poller interface {
	// Poll is supposed to take the bot object
	// subscription channel and start polling
	// for Updates immediately.
	//
	// Poller must listen for stop constantly and close
	// it as soon as it's done polling.
	Poll(b *Bot, updates chan Update, stop chan struct{})
}

// MiddlewarePoller is a special kind of poller that acts
// like a filter for updates. It could be used for spam
// handling, banning or whatever.
//
// For heavy middleware, use increased capacity.
//
type MiddlewarePoller struct {
	Capacity int // Default: 1
	Poller   Poller
	Filter   func(*Update) bool
}

// NewMiddlewarePoller wait for it... constructs a new middleware poller.
func NewMiddlewarePoller(original Poller, filter func(*Update) bool) *MiddlewarePoller {
	return &MiddlewarePoller{
		Poller: original,
		Filter: filter,
	}
}

// Poll sieves updates through middleware filter.
func (p *MiddlewarePoller) Poll(b *Bot, dest chan Update, stop chan struct{}) {
	if p.Capacity < 1 {
		p.Capacity = 1
	}

	middle := make(chan Update, p.Capacity)
	stopPoller := make(chan struct{})

	go p.Poller.Poll(b, middle, stopPoller)

	go func() {
		<-stop
		close(stopPoller)
	}()

	for {
		upd, ok := <-middle
		if !ok {
			close(dest)
			return
		}
		if p.Filter(&upd) {
			dest <- upd
		}
	}
}

// LongPoller is a classic LongPoller with timeout.
type LongPoller struct {
	Limit        int
	Timeout      time.Duration
	LastUpdateID int

	// AllowedUpdates contains the update types
	// you want your bot to receive.
	//
	// Possible values:
	//		message
	// 		edited_message
	// 		channel_post
	// 		edited_channel_post
	// 		inline_query
	// 		chosen_inline_result
	// 		callback_query
	// 		shipping_query
	// 		pre_checkout_query
	// 		poll
	// 		poll_answer
	//
	AllowedUpdates []string
}

// Poll does long polling.
func (p *LongPoller) Poll(b *Bot, dest chan Update, stop chan struct{}) {
	for {
		select {
		case <-stop:
			b.UpdatesWg.Wait()
			close(dest)
			return
		default:
		}

		updates, err := b.getUpdates(p.LastUpdateID+1, p.Limit, p.Timeout, p.AllowedUpdates)
		if err != nil {
			b.debug(err)
			b.debug(ErrCouldNotUpdate)
			continue
		}
		b.UpdatesWg.Add(len(updates))

		for _, update := range updates {
			p.LastUpdateID = update.ID
			dest <- update
		}
	}
}
