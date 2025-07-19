package gotgbotc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type msgRes struct {
	raw json.RawMessage
	err error
}

type msg struct {
	ctx    context.Context
	token  string
	method string
	params map[string]string
	data   map[string]gotgbot.FileReader
	opts   *gotgbot.RequestOpts

	res chan msgRes
}

type RateLimitedBotClient struct {
	base  gotgbot.BotClient
	queue chan msg
}

func (this *RateLimitedBotClient) Start() {
	this.StartWithContext(context.Background())
}

func (this *RateLimitedBotClient) StartWithContext(
	ctx context.Context,
) {
	interval := newInterval(ctx)

	go func() {
		<-ctx.Done()
		close(this.queue)
	}()

	for range interval {
		m, ok := <-this.queue
		if !ok {
			return
		}

		raw, err := this.base.RequestWithContext(
			m.ctx, m.token, m.method, m.params, m.data, m.opts,
		)
		m.res <- msgRes{raw, err}
		close(m.res)
	}
}

func newInterval(ctx context.Context) chan struct{} {
	// TODO: too basic improve this :)
	ret := make(chan struct{})

	go func() {
		ret <- struct{}{}

		for {
			select {
			case <-ctx.Done():
				close(ret)
				return
			case <-time.After(time.Second):
				ret <- struct{}{}
			}
		}
	}()

	return ret
}

func (this *RateLimitedBotClient) RequestWithContext(
	ctx context.Context, token string, method string,
	params map[string]string, data map[string]gotgbot.FileReader,
	opts *gotgbot.RequestOpts,
) (json.RawMessage, error) {
	m := msg{
		ctx, token, method, params, data, opts, make(chan msgRes),
	}

	select {
	case this.queue <- m:
		select {
		case r := <-m.res:
			return r.raw, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (this *RateLimitedBotClient) GetAPIURL(
	opts *gotgbot.RequestOpts,
) string {
	return this.base.GetAPIURL(opts)
}

func (this *RateLimitedBotClient) FileURL(
	token string, tgFilePath string, opts *gotgbot.RequestOpts,
) string {
	return this.base.FileURL(token, tgFilePath, opts)
}

var _ gotgbot.BotClient = (*RateLimitedBotClient)(nil)

func NewRateLimitedBotClient(
	base gotgbot.BotClient,
) *RateLimitedBotClient {
	return &RateLimitedBotClient{base, make(chan msg)}
}
