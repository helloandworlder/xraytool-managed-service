package protocol

import (
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
)

func (u *User) GetTypedAccount() (Account, error) {
	if u.GetAccount() == nil {
		return nil, errors.New("Account is missing").AtWarning()
	}

	rawAccount, err := u.Account.GetInstance()
	if err != nil {
		return nil, err
	}
	if asAccount, ok := rawAccount.(AsAccount); ok {
		return asAccount.AsAccount()
	}
	if account, ok := rawAccount.(Account); ok {
		return account, nil
	}
	return nil, errors.New("Unknown account type: ", u.Account.Type)
}

func (u *User) ToMemoryUser() (*MemoryUser, error) {
	account, err := u.GetTypedAccount()
	if err != nil {
		return nil, err
	}
	// Per-user limits are read from the top-level User fields so every protocol
	// gets them uniformly, regardless of whether its account implements any
	// limits accessor. ToMemoryUser is the single choke point for runtime AddUser.
	return &MemoryUser{
		Account:          account,
		Email:            u.Email,
		Level:            u.Level,
		BandwidthBps:     u.BandwidthBps,
		UplinkLimitBps:   u.UplinkLimitBps,
		DownlinkLimitBps: u.DownlinkLimitBps,
		ConnLimit:        u.ConnLimit,
	}, nil
}

func ToProtoUser(mu *MemoryUser) *User {
	if mu == nil {
		return nil
	}
	u := &User{
		Email:            mu.Email,
		Level:            mu.Level,
		BandwidthBps:     mu.BandwidthBps,
		UplinkLimitBps:   mu.UplinkLimitBps,
		DownlinkLimitBps: mu.DownlinkLimitBps,
		ConnLimit:        mu.ConnLimit,
	}
	// Account is optional: static proxies (socks/http/mixed) carry a user as a
	// limits-only MemoryUser with the password kept beside it, so a user may have
	// no protocol account. Serializing it back out must tolerate that instead of
	// dereferencing a nil account.
	if mu.Account != nil {
		u.Account = serial.ToTypedMessage(mu.Account.ToProto())
	}
	return u
}

// MemoryUser is a parsed form of User, to reduce number of parsing of Account proto.
type MemoryUser struct {
	// Account is the parsed account of the protocol.
	Account Account
	Email   string
	Level   uint32
	// BandwidthBps is retained for older control-plane clients. New callers use
	// the direction-specific fields below.
	BandwidthBps     uint64
	UplinkLimitBps   uint64
	DownlinkLimitBps uint64
	ConnLimit        uint32
}

// DefaultLimitBps is the enforced per-account fallback when a directional
// bandwidth value is absent or zero. Speed limits are never unlimited by
// omission; connection and byte-count limits keep their own zero semantics.
const DefaultLimitBps uint64 = 30_000_000
const DefaultLimitBytesPerSecond uint64 = (DefaultLimitBps + 7) / 8

func (u *MemoryUser) EffectiveUplinkLimitBps() uint64 {
	if u == nil {
		return 0
	}
	if u.UplinkLimitBps > 0 || u.DownlinkLimitBps > 0 {
		if u.UplinkLimitBps > 0 {
			return u.UplinkLimitBps
		}
		return DefaultLimitBps
	}
	if u.BandwidthBps > 0 {
		return u.BandwidthBps
	}
	return DefaultLimitBps
}

func (u *MemoryUser) EffectiveDownlinkLimitBps() uint64 {
	if u == nil {
		return 0
	}
	if u.UplinkLimitBps > 0 || u.DownlinkLimitBps > 0 {
		if u.DownlinkLimitBps > 0 {
			return u.DownlinkLimitBps
		}
		return DefaultLimitBps
	}
	if u.BandwidthBps > 0 {
		return u.BandwidthBps
	}
	return DefaultLimitBps
}
