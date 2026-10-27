package wg

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

type KeyPair struct {
	Private string
	Public  string
}

func NewKeyPair() (KeyPair, error) {
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Private: k.String(), Public: k.PublicKey().String()}, nil
}

func NewPresharedKey() (string, error) {
	k, err := wgtypes.GenerateKey()
	if err != nil {
		return "", err
	}
	return k.String(), nil
}

func PublicKeyOf(private string) (string, error) {
	k, err := wgtypes.ParseKey(private)
	if err != nil {
		return "", err
	}
	return k.PublicKey().String(), nil
}

func ValidKey(s string) bool {
	_, err := wgtypes.ParseKey(s)
	return err == nil
}
