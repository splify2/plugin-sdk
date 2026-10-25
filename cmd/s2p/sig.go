package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Подпись Ed25519 в формате usign/signify (docs/s2p.md, «Подпись»):
//   открытый ключ — "Ed", номер (8), ключ (32)           = 42 байта в base64
//   подпись       — "Ed", номер (8), подпись (64)         = 74 байта в base64
//   закрытый ключ — "Ed","BK", раунды (4, BE), соль (16), контрольная сумма (8), номер (8), ключ (64)
// Закрытый ключ без пароля (раунды 0) — как `usign -G -N`; с паролем (bcrypt_pbkdf) не читается.

type keyNum [8]byte

func (k keyNum) String() string { return hex.EncodeToString(k[:]) }

type pubKey struct {
	num keyNum
	key ed25519.PublicKey
}

type secKey struct {
	num keyNum
	key ed25519.PrivateKey
}

type sigBlock struct {
	num keyNum
	sig []byte
}

const commentPrefix = "untrusted comment:"

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// pairs разбирает файл на пары «комментарий, base64».
func pairs(text []byte) ([][]byte, error) {
	var out [][]byte
	lines := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}
		if !strings.HasPrefix(l, commentPrefix) {
			return nil, fmt.Errorf("строка %d: ожидался «untrusted comment: …»", i+1)
		}
		i++
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i >= len(lines) {
			return nil, fmt.Errorf("после комментария нет данных")
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[i]))
		if err != nil {
			return nil, fmt.Errorf("строка %d: base64: %v", i+1, err)
		}
		out = append(out, raw)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("файл пуст")
	}
	return out, nil
}

func parsePub(text []byte) (pubKey, error) {
	var k pubKey
	ps, err := pairs(text)
	if err != nil {
		return k, err
	}
	raw := ps[0]
	if len(raw) != 42 || raw[0] != 'E' || raw[1] != 'd' {
		return k, fmt.Errorf("не открытый ключ Ed25519 usign (нужно 42 байта «Ed…»)")
	}
	copy(k.num[:], raw[2:10])
	k.key = ed25519.PublicKey(append([]byte(nil), raw[10:]...))
	return k, nil
}

func parseSec(text []byte) (secKey, error) {
	var k secKey
	ps, err := pairs(text)
	if err != nil {
		return k, err
	}
	raw := ps[0]
	if len(raw) != 104 || raw[0] != 'E' || raw[1] != 'd' || raw[2] != 'B' || raw[3] != 'K' {
		return k, fmt.Errorf("не закрытый ключ Ed25519 usign (нужно 104 байта «EdBK…»)")
	}
	if raw[4]|raw[5]|raw[6]|raw[7] != 0 {
		return k, fmt.Errorf("закрытый ключ зашифрован паролем; s2p читает ключи без пароля (usign -G -N или s2p keygen)")
	}
	sk := raw[40:]
	sum := sha512.Sum512(sk)
	if !bytes.Equal(sum[:8], raw[24:32]) {
		return k, fmt.Errorf("контрольная сумма закрытого ключа не сходится")
	}
	copy(k.num[:], raw[32:40])
	k.key = ed25519.PrivateKey(append([]byte(nil), sk...))
	return k, nil
}

func (k secKey) pub() pubKey {
	return pubKey{num: k.num, key: k.key.Public().(ed25519.PublicKey)}
}

func formatPub(k pubKey, comment string) []byte {
	raw := append(append([]byte("Ed"), k.num[:]...), k.key...)
	return []byte(commentPrefix + " " + comment + "\n" + b64(raw) + "\n")
}

func formatSec(k secKey, comment string) []byte {
	raw := make([]byte, 0, 104)
	raw = append(raw, 'E', 'd', 'B', 'K', 0, 0, 0, 0)
	raw = append(raw, make([]byte, 16)...)
	sum := sha512.Sum512(k.key)
	raw = append(raw, sum[:8]...)
	raw = append(raw, k.num[:]...)
	raw = append(raw, k.key...)
	return []byte(commentPrefix + " " + comment + "\n" + b64(raw) + "\n")
}

func genKey() (secKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return secKey{}, err
	}
	_ = pub
	var k secKey
	k.key = priv
	if _, err := rand.Read(k.num[:]); err != nil {
		return k, err
	}
	return k, nil
}

func (k secKey) sign(msg []byte) sigBlock {
	return sigBlock{num: k.num, sig: ed25519.Sign(k.key, msg)}
}

func formatSigs(bs []sigBlock) []byte {
	var out bytes.Buffer
	for _, b := range bs {
		raw := append(append([]byte("Ed"), b.num[:]...), b.sig...)
		fmt.Fprintf(&out, "%s signature from key %s\n%s\n", commentPrefix, b.num, b64(raw))
	}
	return out.Bytes()
}

func parseSigs(text []byte) ([]sigBlock, error) {
	ps, err := pairs(text)
	if err != nil {
		return nil, err
	}
	var out []sigBlock
	for i, raw := range ps {
		if len(raw) != 74 || raw[0] != 'E' || raw[1] != 'd' {
			return nil, fmt.Errorf("блок подписи %d: не Ed25519 usign (нужно 74 байта «Ed…»)", i+1)
		}
		var b sigBlock
		copy(b.num[:], raw[2:10])
		b.sig = append([]byte(nil), raw[10:]...)
		out = append(out, b)
	}
	return out, nil
}

// trust — доверенные ключи и отозванные номера.
type trust struct {
	keys    map[keyNum]pubKey
	revoked map[keyNum]bool
}

func newTrust() *trust { return &trust{keys: map[keyNum]pubKey{}, revoked: map[keyNum]bool{}} }

func (t *trust) addFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	k, err := parsePub(b)
	if err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	t.keys[k.num] = k
	return nil
}

// addDir берёт каталог ключей ролей: <dir>/<роль>/<номер>.pub (раскладка /usr/share/steer/keys).
func (t *trust) addRole(dir, role string) error {
	fs, err := filepath.Glob(filepath.Join(dir, role, "*.pub"))
	if err != nil {
		return err
	}
	for _, f := range fs {
		if err := t.addFile(f); err != nil {
			return err
		}
	}
	return nil
}

func (t *trust) addRevoked(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, l := range strings.Fields(string(b)) {
		raw, err := hex.DecodeString(l)
		if err != nil || len(raw) != 8 {
			return fmt.Errorf("%s: %q не номер ключа (16 знаков hex)", path, l)
		}
		var n keyNum
		copy(n[:], raw)
		t.revoked[n] = true
	}
	return nil
}

// verify годится, если хоть один блок подписан доверенным неотозванным ключом и сверяется.
// what — что подписано, для отказа («MANIFEST vless»).
func (t *trust) verify(what string, msg, sigText []byte) error {
	bs, err := parseSigs(sigText)
	if err != nil {
		return fmt.Errorf("%s: подпись не читается: %v", what, err)
	}
	var seen []string
	bad := ""
	for _, b := range bs {
		seen = append(seen, b.num.String())
		if t.revoked[b.num] {
			continue
		}
		k, ok := t.keys[b.num]
		if !ok {
			continue
		}
		if ed25519.Verify(k.key, msg, b.sig) {
			return nil
		}
		if bad == "" {
			bad = b.num.String()
		}
	}
	if bad != "" {
		return fmt.Errorf("%s: подпись ключа %s не сходится — файл изменён или подпись испорчена", what, bad)
	}
	for _, b := range bs {
		if t.revoked[b.num] {
			return fmt.Errorf("%s подписан ключом %s, он отозван", what, b.num)
		}
	}
	return fmt.Errorf("%s подписан ключом %s, такого ключа среди доверенных нет", what, strings.Join(seen, ", "))
}
