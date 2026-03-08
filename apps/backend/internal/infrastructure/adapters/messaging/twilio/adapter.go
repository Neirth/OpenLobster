// Copyright (C) 2024 OpenLobster contributors
// SPDX-License-Identifier: see LICENSE
package twilio

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/google/uuid"

	twilioclient "github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// Adapter implements ports.MessagingPort for the Twilio SMS/MMS platform.
type Adapter struct {
	accountSID   string
	fromNumber   string
	twilioClient *twilioclient.RestClient
}

// NewAdapter creates a new Twilio adapter backed by the official twilio-go SDK.
func NewAdapter(accountSID, authToken, fromNumber string) *Adapter {
	client := twilioclient.NewRestClientWithParams(twilioclient.ClientParams{
		Username: accountSID,
		Password: authToken,
	})
	return &Adapter{
		accountSID:   accountSID,
		fromNumber:   fromNumber,
		twilioClient: client,
	}
}

// SendTyping is a no-op for Twilio SMS (no typing indicator).
func (a *Adapter) SendTyping(_ context.Context, _ string) error { return nil }

// SendMessage sends an SMS via the Twilio REST API.
func (a *Adapter) SendMessage(ctx context.Context, msg *models.Message) error {
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(msg.ChannelID)
	params.SetFrom(a.fromNumber)
	params.SetBody(msg.Content)
	_, err := a.twilioClient.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("twilio send message: %w", err)
	}
	return nil
}

// SendMedia sends an MMS with an optional media URL via the Twilio REST API.
func (a *Adapter) SendMedia(ctx context.Context, media *ports.Media) error {
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(media.ChatID)
	params.SetFrom(a.fromNumber)
	if media.Caption != "" {
		params.SetBody(media.Caption)
	}
	if media.URL != "" {
		params.SetMediaUrl([]string{media.URL})
	}
	_, err := a.twilioClient.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("twilio send media: %w", err)
	}
	return nil
}

func (a *Adapter) HandleWebhook(ctx context.Context, payload []byte) (*models.Message, error) {
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return nil, err
	}

	msg := &models.Message{
		ID:        uuid.New(),
		ChannelID: values.Get("From"),
		Content:   values.Get("Body"),
	}

	return msg, nil
}

func (a *Adapter) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	return &ports.UserInfo{
		ID:          userID,
		Username:    userID,
		DisplayName: userID,
	}, nil
}

func (a *Adapter) React(ctx context.Context, messageID string, emoji string) error {
	return nil
}

type VoiceAdapter struct {
	*Adapter
}

func NewVoiceAdapter(accountSID, authToken, fromNumber string) *VoiceAdapter {
	return &VoiceAdapter{
		Adapter: NewAdapter(accountSID, authToken, fromNumber),
	}
}

func (a *VoiceAdapter) AcceptCall(ctx context.Context, callID string) (*ports.VoiceCall, error) {
	return &ports.VoiceCall{
		ID:        callID,
		Status:    ports.CallStatusActive,
		StartTime: 0,
	}, nil
}

func (a *VoiceAdapter) EndCall(ctx context.Context, callID string) error {
	return nil
}

func (a *VoiceAdapter) StartStream(ctx context.Context, callID string) (*ports.VoiceStream, error) {
	input := make(chan ports.AudioChunk, 10)
	output := make(chan ports.AudioChunk, 10)
	interrupt := make(chan struct{}, 1)
	mute := make(chan bool, 1)

	return &ports.VoiceStream{
		Input:     input,
		Output:    output,
		Interrupt: interrupt,
		Mute:      mute,
	}, nil
}

func (a *VoiceAdapter) Interrupt(ctx context.Context, callID string) error {
	return nil
}

func (a *VoiceAdapter) SendTone(ctx context.Context, callID string, tone ports.ToneType) error {
	return nil
}

func (a *VoiceAdapter) SupportsVoiceCalls() bool {
	return true
}

func GenerateTwiMLResponse(say string, gather bool) string {
	resp := TwiMLResponse{}
	if gather {
		resp.Gather = &Gather{
			NumDigits: "1",
			Action:    "/voice/gather",
			Say:       Say{Text: say},
		}
	} else {
		resp.Say = &Say{Text: say}
	}
	data, _ := xml.Marshal(resp)
	return string(data)
}

type TwiMLResponse struct {
	XMLName xml.Name `xml:"Response"`
	Say     *Say     `xml:"Say,omitempty"`
	Gather  *Gather  `xml:"Gather,omitempty"`
}

type Say struct {
	Text string `xml:",chardata"`
}

type Gather struct {
	NumDigits string `xml:"numDigits,attr"`
	Action    string `xml:"action,attr"`
	Say       Say    `xml:"Say"`
}

var _ ports.MessagingPort = (*Adapter)(nil)
var _ ports.VoicePort = (*VoiceAdapter)(nil)

func (a *Adapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{
		HasVoiceMessage: true,
		HasCallStream:   true,
		HasTextStream:   true,
		HasMediaSupport: true,
	}
}

// Start is a no-op for Twilio: messages arrive via the incoming webhook endpoint.
func (a *Adapter) Start(_ context.Context, _ func(context.Context, *models.Message)) error {
	return nil
}

func (a *Adapter) ConvertAudioForPlatform(ctx context.Context, audioData []byte, format string) ([]byte, string, error) {
	return audioData, "mp3", nil
}
