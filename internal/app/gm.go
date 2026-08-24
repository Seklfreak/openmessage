package app

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.mau.fi/mautrix-gmessages/pkg/libgm/gmproto"

	"github.com/maxghenis/openmessage/internal/client"
)

var (
	getGoogleConversationForSend = func(a *App, conversationID string) (*gmproto.Conversation, error) {
		cli := a.GetClient()
		if cli == nil {
			return nil, fmt.Errorf(ErrNotConnected)
		}
		return cli.GM.GetConversation(client.GMContext(), conversationID)
	}
	sendGoogleTextPayload = func(a *App, payload *gmproto.SendMessageRequest) (*gmproto.SendMessageResponse, error) {
		cli := a.GetClient()
		if cli == nil {
			return nil, fmt.Errorf(ErrNotConnected)
		}
		return cli.GM.SendMessage(client.GMContext(), payload)
	}
)

// ErrNotConnected is the error message returned when an operation requires
// a Google Messages connection but one is not established.
const ErrNotConnected = "not connected to Google Messages"

// ContactNumberMysteriousInt is the default value for the MysteriousInt field
// in ContactNumber structs used for conversation lookups and message sending.
const ContactNumberMysteriousInt = 7

// NewContactNumbers builds a ContactNumber slice from phone number strings,
// suitable for GetOrCreateConversation requests.
func NewContactNumbers(phones []string) []*gmproto.ContactNumber {
	numbers := make([]*gmproto.ContactNumber, len(phones))
	for i, phone := range phones {
		numbers[i] = &gmproto.ContactNumber{
			MysteriousInt: ContactNumberMysteriousInt,
			Number:        phone,
			Number2:       phone,
		}
	}
	return numbers
}

// ExtractSIMAndParticipant finds the current user's participant ID and SIM
// payload from a conversation, falling back to the conversation's SIM card.
func ExtractSIMAndParticipant(conv *gmproto.Conversation) (participantID string, sim *gmproto.SIMPayload) {
	for _, p := range conv.GetParticipants() {
		if p.GetIsMe() {
			if id := p.GetID(); id != nil {
				participantID = id.GetNumber()
			}
			sim = p.GetSimPayload()
			break
		}
	}
	if sim == nil {
		if sc := conv.GetSimCard(); sc != nil {
			sim = sc.GetSIMData().GetSIMPayload()
		}
	}
	return
}

// BuildSendPayload constructs a SendMessageRequest matching the format used by
// the mautrix bridge: MessageInfo array (not MessagePayloadContent), TmpID in 3
// places, SIMPayload, and ParticipantID.
func BuildSendPayload(conversationID, message, replyToID, participantID string, sim *gmproto.SIMPayload) *gmproto.SendMessageRequest {
	return BuildSendPayloadWithTmpID(conversationID, message, replyToID, participantID, sim, "")
}

// sendTmpIDNamespace scopes the UUIDv5 derivation in newSendTmpID. It is an
// arbitrary fixed UUID — only its stability matters, so never regenerate it:
// changing it would give in-flight retries a different tmpID than their
// original send and cost the server-side dedup below.
var sendTmpIDNamespace = uuid.MustParse("6d9c4a66-641a-4701-b4fc-f2d7e273af24")

// newSendTmpID returns the tmpID to put on the wire for one send.
//
// Google Messages' own app uses UUID-format tmpIDs (upstream libgm followed
// suit in mautrix fa79aa84), so every send from here does too. A caller-owned
// key — an HTTP idempotency_key, a queued send's request id — is hashed into a
// UUIDv5 rather than sent verbatim: the derivation is deterministic across
// processes and restarts, so a retry carrying the same key still produces the
// same tmpID and Google still dedups it, while the key itself stays a local
// identifier in outgoing_sends.
func newSendTmpID(preferred string) string {
	if preferred = strings.TrimSpace(preferred); preferred != "" {
		return uuid.NewSHA1(sendTmpIDNamespace, []byte(preferred)).String()
	}
	return uuid.NewString()
}

// BuildSendPayloadWithTmpID is BuildSendPayload with an optional caller-owned
// temporary ID. Queued sends use this to make retries idempotent server-side.
func BuildSendPayloadWithTmpID(conversationID, message, replyToID, participantID string, sim *gmproto.SIMPayload, tmpID string) *gmproto.SendMessageRequest {
	tmpID = newSendTmpID(tmpID)
	req := &gmproto.SendMessageRequest{
		ConversationID: conversationID,
		MessagePayload: &gmproto.MessagePayload{
			TmpID:                 tmpID,
			MessagePayloadContent: nil,
			MessageInfo: []*gmproto.MessageInfo{{
				Data: &gmproto.MessageInfo_MessageContent{MessageContent: &gmproto.MessageContent{
					Content: message,
				}},
			}},
			ConversationID: conversationID,
			ParticipantID:  participantID,
			TmpID2:         tmpID,
		},
		SIMPayload: sim,
		TmpID:      tmpID,
	}
	if replyToID != "" {
		req.Reply = &gmproto.ReplyPayload{
			MessageID: replyToID,
		}
	}
	return req
}

// BuildSendMediaPayload constructs a SendMessageRequest with a MediaContent attachment
// instead of text. Uses the same MessageInfo array format as BuildSendPayload.
func BuildSendMediaPayload(conversationID string, media *gmproto.MediaContent, participantID string, sim *gmproto.SIMPayload) *gmproto.SendMessageRequest {
	return BuildSendMediaPayloadWithTmpID(conversationID, media, participantID, sim, "")
}

// BuildSendMediaPayloadWithTmpID is BuildSendMediaPayload with an optional
// caller-owned temporary ID for idempotent queued media retries.
func BuildSendMediaPayloadWithTmpID(conversationID string, media *gmproto.MediaContent, participantID string, sim *gmproto.SIMPayload, tmpID string) *gmproto.SendMessageRequest {
	tmpID = newSendTmpID(tmpID)
	return &gmproto.SendMessageRequest{
		ConversationID: conversationID,
		MessagePayload: &gmproto.MessagePayload{
			TmpID:                 tmpID,
			MessagePayloadContent: nil,
			MessageInfo: []*gmproto.MessageInfo{{
				Data: &gmproto.MessageInfo_MediaContent{MediaContent: media},
			}},
			ConversationID: conversationID,
			ParticipantID:  participantID,
			TmpID2:         tmpID,
		},
		SIMPayload: sim,
		TmpID:      tmpID,
	}
}

// BuildReactionPayload constructs a SendReactionRequest using
// gmproto.MakeReactionData for proper emoji type mapping.
func BuildReactionPayload(messageID, emoji, action string, sim *gmproto.SIMPayload) *gmproto.SendReactionRequest {
	var a gmproto.SendReactionRequest_Action
	switch strings.ToLower(action) {
	case "remove":
		a = gmproto.SendReactionRequest_REMOVE
	case "switch":
		a = gmproto.SendReactionRequest_SWITCH
	default:
		a = gmproto.SendReactionRequest_ADD
	}
	return &gmproto.SendReactionRequest{
		MessageID:    messageID,
		ReactionData: gmproto.MakeReactionData(emoji),
		Action:       a,
		SIMPayload:   sim,
	}
}
