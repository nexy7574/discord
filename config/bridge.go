// mautrix-discord - A Matrix-Discord puppeting bridge.
// Copyright (C) 2022 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package config

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/bwmarrin/discordgo"

	"maunium.net/go/mautrix/bridge/bridgeconfig"
)

type BridgeConfig struct {
	UsernameTemplate    string `yaml:"username_template"`
	DisplaynameTemplate string `yaml:"displayname_template"`
	ChannelnameTemplate string `yaml:"channelname_template"`

	DeliveryReceipts    bool `yaml:"delivery_receipts"`
	MessageStatusEvents bool `yaml:"message_status_events"`
	MessageErrorNotices bool `yaml:"message_error_notices"`
	RestrictedRooms     bool `yaml:"restricted_rooms"`

	CommandPrefix string `yaml:"command_prefix"`

	ManagementRoomText bridgeconfig.ManagementRoomTexts `yaml:"management_room_text"`

	PortalMessageBuffer int `yaml:"portal_message_buffer"`

	SyncDirectChatList bool `yaml:"sync_direct_chat_list"`
	ResendBridgeInfo   bool `yaml:"resend_bridge_info"`
	FederateRooms      bool `yaml:"federate_rooms"`

	DoublePuppetServerMap      map[string]string `yaml:"double_puppet_server_map"`
	DoublePuppetAllowDiscovery bool              `yaml:"double_puppet_allow_discovery"`
	LoginSharedSecretMap       map[string]string `yaml:"login_shared_secret_map"`

	Encryption bridgeconfig.EncryptionConfig `yaml:"encryption"`

	Provisioning struct {
		Prefix       string `yaml:"prefix"`
		SharedSecret string `yaml:"shared_secret"`
	} `yaml:"provisioning"`

	Permissions bridgeconfig.PermissionConfig `yaml:"permissions"`

	usernameTemplate    *template.Template `yaml:"-"`
	displaynameTemplate *template.Template `yaml:"-"`
	channelnameTemplate *template.Template `yaml:"-"`
}

func (bc *BridgeConfig) GetResendBridgeInfo() bool {
	return bc.ResendBridgeInfo
}

func (bc *BridgeConfig) EnableMessageStatusEvents() bool {
	return bc.MessageStatusEvents
}

func (bc *BridgeConfig) EnableMessageErrorNotices() bool {
	return bc.MessageErrorNotices
}

func boolToInt(val bool) int {
	if val {
		return 1
	}
	return 0
}

func (bc *BridgeConfig) Validate() error {
	_, hasWildcard := bc.Permissions["*"]
	_, hasExampleDomain := bc.Permissions["example.com"]
	_, hasExampleUser := bc.Permissions["@admin:example.com"]
	exampleLen := boolToInt(hasWildcard) + boolToInt(hasExampleUser) + boolToInt(hasExampleDomain)
	if len(bc.Permissions) <= exampleLen {
		return errors.New("bridge.permissions not configured")
	}
	return nil
}

type umBridgeConfig BridgeConfig

func (bc *BridgeConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	err := unmarshal((*umBridgeConfig)(bc))
	if err != nil {
		return err
	}

	bc.usernameTemplate, err = template.New("username").Parse(bc.UsernameTemplate)
	if err != nil {
		return err
	} else if !strings.Contains(bc.FormatUsername("1234567890"), "1234567890") {
		return fmt.Errorf("username template is missing user ID placeholder")
	}

	bc.displaynameTemplate, err = template.New("displayname").Parse(bc.DisplaynameTemplate)
	if err != nil {
		return err
	}

	bc.channelnameTemplate, err = template.New("channelname").Parse(bc.ChannelnameTemplate)
	if err != nil {
		return err
	}

	return nil
}

var _ bridgeconfig.BridgeConfig = (*BridgeConfig)(nil)

func (bc BridgeConfig) GetEncryptionConfig() bridgeconfig.EncryptionConfig {
	return bc.Encryption
}

func (bc BridgeConfig) GetCommandPrefix() string {
	return bc.CommandPrefix
}

func (bc BridgeConfig) GetManagementRoomTexts() bridgeconfig.ManagementRoomTexts {
	return bc.ManagementRoomText
}

func (bc BridgeConfig) FormatUsername(userid string) string {
	var buffer strings.Builder
	_ = bc.usernameTemplate.Execute(&buffer, userid)
	return buffer.String()
}

func (bc BridgeConfig) FormatDisplayname(user *discordgo.User) string {
	var buffer strings.Builder
	_ = bc.displaynameTemplate.Execute(&buffer, user)
	return buffer.String()
}

type wrappedChannel struct {
	*discordgo.Channel
	Guild  string
	Folder string
}

func (bc BridgeConfig) FormatChannelname(channel *discordgo.Channel, session *discordgo.Session) (string, error) {
	var buffer strings.Builder
	var guildName, folderName string

	if channel.Type != discordgo.ChannelTypeDM && channel.Type != discordgo.ChannelTypeGroupDM {
		guild, err := session.Guild(channel.GuildID)
		if err != nil {
			return "", fmt.Errorf("find guild: %w", err)
		}
		guildName = guild.Name

		folder, err := session.Channel(channel.ParentID)
		if err == nil {
			folderName = folder.Name
		}
	} else {
		// Group DM's can have a name, but DM's can't, so if we didn't get a
		// name return a comma separated list of the formatted user names.
		if channel.Name == "" {
			recipients := make([]string, len(channel.Recipients))
			for idx, user := range channel.Recipients {
				recipients[idx] = bc.FormatDisplayname(user)
			}

			return strings.Join(recipients, ", "), nil
		}
	}

	_ = bc.channelnameTemplate.Execute(&buffer, wrappedChannel{
		Channel: channel,
		Guild:   guildName,
		Folder:  folderName,
	})

	return buffer.String(), nil
}
