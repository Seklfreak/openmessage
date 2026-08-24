package google

import (
	"go.mau.fi/mautrix-gmessages/pkg/libgm"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"

	"github.com/maxghenis/openmessage/internal/client"
)

// gmCompat adapts *libgm.Client to this package's context-free seams, so the
// seams and their test fakes stay unchanged now that libgm takes a context on
// every phone request. See client.GMContext for why that context is background.
type gmCompat struct {
	gm *libgm.Client
}

func newGMCompat(gm *libgm.Client) *gmCompat {
	return &gmCompat{gm: gm}
}

func (c *gmCompat) SetEventHandler(handler libgm.EventHandler) { c.gm.SetEventHandler(handler) }

func (c *gmCompat) Connect() error { return c.gm.Connect() }

func (c *gmCompat) Disconnect() { c.gm.Disconnect() }

func (c *gmCompat) NotifyDittoActivity() (<-chan *libgm.IncomingRPCMessage, error) {
	return c.gm.NotifyDittoActivity(client.GMContext())
}

func (c *gmCompat) GetConversation(conversationID string) (*gmproto.Conversation, error) {
	return c.gm.GetConversation(client.GMContext(), conversationID)
}

func (c *gmCompat) SendMessage(payload *gmproto.SendMessageRequest) (*gmproto.SendMessageResponse, error) {
	return c.gm.SendMessage(client.GMContext(), payload)
}

func (c *gmCompat) SendReaction(payload *gmproto.SendReactionRequest) (*gmproto.SendReactionResponse, error) {
	return c.gm.SendReaction(client.GMContext(), payload)
}

func (c *gmCompat) MarkRead(conversationID, messageID string) error {
	return c.gm.MarkRead(client.GMContext(), conversationID, messageID)
}

func (c *gmCompat) UploadMedia(data []byte, filename, mime string) (*gmproto.MediaContent, error) {
	return c.gm.UploadMedia(data, filename, mime)
}
