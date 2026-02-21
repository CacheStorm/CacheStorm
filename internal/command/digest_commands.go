package command

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"hash"
	"strconv"
	"strings"
)

func RegisterDigestCommands(router *Router) {
	router.Register(&CommandDef{Name: "DIGEST.MD5", Handler: cmdDIGESTMD5})
	router.Register(&CommandDef{Name: "DIGEST.SHA1", Handler: cmdDIGESTSHA1})
	router.Register(&CommandDef{Name: "DIGEST.SHA256", Handler: cmdDIGESTSHA256})
	router.Register(&CommandDef{Name: "DIGEST.SHA512", Handler: cmdDIGESTSHA512})
	router.Register(&CommandDef{Name: "DIGEST.HMAC", Handler: cmdDIGESTHMAC})
	router.Register(&CommandDef{Name: "DIGEST.HMACMD5", Handler: cmdDIGESTHMACMD5})
	router.Register(&CommandDef{Name: "DIGEST.HMACSHA1", Handler: cmdDIGESTHMACSHA1})
	router.Register(&CommandDef{Name: "DIGEST.HMACSHA256", Handler: cmdDIGESTHMACSHA256})
	router.Register(&CommandDef{Name: "DIGEST.HMACSHA512", Handler: cmdDIGESTHMACSHA512})
	router.Register(&CommandDef{Name: "DIGEST.CRC32", Handler: cmdDIGESTCRC32})
	router.Register(&CommandDef{Name: "DIGEST.ADLER32", Handler: cmdDIGESTADLER32})
	router.Register(&CommandDef{Name: "DIGEST.BASE64ENCODE", Handler: cmdDIGESTBASE64ENCODE})
	router.Register(&CommandDef{Name: "DIGEST.BASE64DECODE", Handler: cmdDIGESTBASE64DECODE})
	router.Register(&CommandDef{Name: "DIGEST.HEXENCODE", Handler: cmdDIGESTHEXENCODE})
	router.Register(&CommandDef{Name: "DIGEST.HEXDECODE", Handler: cmdDIGESTHEXDECODE})
	router.Register(&CommandDef{Name: "CRYPTO.HASH", Handler: cmdCRYPTOHASH})
	router.Register(&CommandDef{Name: "CRYPTO.HMAC", Handler: cmdCRYPTOHMAC})
}

func cmdDIGESTMD5(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	hash := md5.Sum(data)
	return ctx.WriteBulkString(hex.EncodeToString(hash[:]))
}

func cmdDIGESTSHA1(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	hash := sha1.Sum(data)
	return ctx.WriteBulkString(hex.EncodeToString(hash[:]))
}

func cmdDIGESTSHA256(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	hash := sha256.Sum256(data)
	return ctx.WriteBulkString(hex.EncodeToString(hash[:]))
}

func cmdDIGESTSHA512(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	hash := sha512.Sum512(data)
	return ctx.WriteBulkString(hex.EncodeToString(hash[:]))
}

func cmdDIGESTHMAC(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	algo := strings.ToUpper(ctx.ArgString(0))
	key := ctx.Arg(1)
	data := ctx.Arg(2)

	var h hash.Hash
	switch algo {
	case "MD5":
		h = hmac.New(md5.New, key)
	case "SHA1", "SHA-1":
		h = hmac.New(sha1.New, key)
	case "SHA256", "SHA-256":
		h = hmac.New(sha256.New, key)
	case "SHA512", "SHA-512":
		h = hmac.New(sha512.New, key)
	default:
		return ctx.WriteError(ErrInvalidAlgorithm)
	}

	h.Write(data)
	return ctx.WriteBulkString(hex.EncodeToString(h.Sum(nil)))
}

func cmdDIGESTHMACMD5(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.Arg(0)
	data := ctx.Arg(1)

	h := hmac.New(md5.New, key)
	h.Write(data)
	return ctx.WriteBulkString(hex.EncodeToString(h.Sum(nil)))
}

func cmdDIGESTHMACSHA1(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.Arg(0)
	data := ctx.Arg(1)

	h := hmac.New(sha1.New, key)
	h.Write(data)
	return ctx.WriteBulkString(hex.EncodeToString(h.Sum(nil)))
}

func cmdDIGESTHMACSHA256(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.Arg(0)
	data := ctx.Arg(1)

	h := hmac.New(sha256.New, key)
	h.Write(data)
	return ctx.WriteBulkString(hex.EncodeToString(h.Sum(nil)))
}

func cmdDIGESTHMACSHA512(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.Arg(0)
	data := ctx.Arg(1)

	h := hmac.New(sha512.New, key)
	h.Write(data)
	return ctx.WriteBulkString(hex.EncodeToString(h.Sum(nil)))
}

func cmdDIGESTCRC32(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	crc := crc32Checksum(data)
	return ctx.WriteInteger(int64(crc))
}

func cmdDIGESTADLER32(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	adler := adler32Checksum(data)
	return ctx.WriteInteger(int64(adler))
}

func cmdDIGESTBASE64ENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	encoded := base64Encode(data)
	return ctx.WriteBulkString(encoded)
}

func cmdDIGESTBASE64DECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	encoded := ctx.ArgString(0)
	decoded, err := base64Decode(encoded)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(string(decoded))
}

func cmdDIGESTHEXENCODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	data := ctx.Arg(0)
	return ctx.WriteBulkString(hex.EncodeToString(data))
}

func cmdDIGESTHEXDECODE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	encoded := ctx.ArgString(0)
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(string(decoded))
}

func cmdCRYPTOHASH(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	algo := strings.ToUpper(ctx.ArgString(0))
	data := ctx.Arg(1)

	var result string
	switch algo {
	case "MD5":
		hash := md5.Sum(data)
		result = hex.EncodeToString(hash[:])
	case "SHA1", "SHA-1":
		hash := sha1.Sum(data)
		result = hex.EncodeToString(hash[:])
	case "SHA256", "SHA-256":
		hash := sha256.Sum256(data)
		result = hex.EncodeToString(hash[:])
	case "SHA512", "SHA-512":
		hash := sha512.Sum512(data)
		result = hex.EncodeToString(hash[:])
	case "SHA384", "SHA-384":
		hash := sha512.Sum384(data)
		result = hex.EncodeToString(hash[:])
	case "SHA512_224", "SHA-512/224":
		hash := sha512.Sum512_224(data)
		result = hex.EncodeToString(hash[:])
	case "SHA512_256", "SHA-512/256":
		hash := sha512.Sum512_256(data)
		result = hex.EncodeToString(hash[:])
	default:
		return ctx.WriteError(ErrInvalidAlgorithm)
	}

	return ctx.WriteBulkString(result)
}

func cmdCRYPTOHMAC(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	algo := strings.ToUpper(ctx.ArgString(0))
	key := ctx.Arg(1)
	data := ctx.Arg(2)

	var h hash.Hash
	switch algo {
	case "MD5":
		h = hmac.New(md5.New, key)
	case "SHA1", "SHA-1":
		h = hmac.New(sha1.New, key)
	case "SHA256", "SHA-256":
		h = hmac.New(sha256.New, key)
	case "SHA384", "SHA-384":
		h = hmac.New(sha512.New384, key)
	case "SHA512", "SHA-512":
		h = hmac.New(sha512.New, key)
	default:
		return ctx.WriteError(ErrInvalidAlgorithm)
	}

	h.Write(data)
	return ctx.WriteBulkString(hex.EncodeToString(h.Sum(nil)))
}

func crc32Checksum(data []byte) uint32 {
	const poly = 0xEDB88320
	var crc uint32 = 0xFFFFFFFF

	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
	}

	return crc ^ 0xFFFFFFFF
}

func adler32Checksum(data []byte) uint32 {
	var a, b uint32 = 1, 0
	const mod = 65521

	for _, d := range data {
		a = (a + uint32(d)) % mod
		b = (b + a) % mod
	}

	return (b << 16) | a
}

func base64Encode(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	result := make([]byte, 0, (len(data)+2)/3*4)

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i

		if remaining >= 3 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				base64Chars[n&0x3F],
			)
		} else if remaining == 2 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				'=',
			)
		} else {
			n = uint32(data[i]) << 16
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				'=',
				'=',
			)
		}
	}

	return string(result)
}

func base64Decode(encoded string) ([]byte, error) {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	decodeMap := make(map[byte]int)
	for i := 0; i < 64; i++ {
		decodeMap[base64Chars[i]] = i
	}

	encoded = strings.ReplaceAll(encoded, "=", "")
	encoded = strings.ReplaceAll(encoded, "\n", "")
	encoded = strings.ReplaceAll(encoded, "\r", "")
	encoded = strings.ReplaceAll(encoded, " ", "")

	if len(encoded)%4 != 0 {
		return nil, ErrInvalidBase64
	}

	result := make([]byte, 0, len(encoded)*3/4)

	for i := 0; i < len(encoded); i += 4 {
		var n uint32
		padding := 0

		for j := 0; j < 4 && i+j < len(encoded); j++ {
			c := encoded[i+j]
			if c == '=' {
				padding++
				continue
			}
			val, ok := decodeMap[c]
			if !ok {
				return nil, ErrInvalidBase64
			}
			n |= uint32(val) << uint(18-j*6)
		}

		result = append(result, byte(n>>16&0xFF))
		if padding < 2 {
			result = append(result, byte(n>>8&0xFF))
		}
		if padding < 1 {
			result = append(result, byte(n&0xFF))
		}
	}

	return result, nil
}

var ErrInvalidAlgorithm = errors.New("ERR invalid algorithm")
var ErrInvalidBase64 = errors.New("ERR invalid base64 encoding")

func init() {
	_ = strconv.Itoa(0)
	_ = strings.ToLower("")
}
