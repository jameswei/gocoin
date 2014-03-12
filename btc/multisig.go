package btc

import (
	"fmt"
	"bytes"
	"errors"
)

type MultiSig struct {
	SigsNeeded uint
	Signatures []*Signature
	PublicKeys [][]byte
}


func NewMultiSig(n uint) (res *MultiSig) {
	res = new(MultiSig)
	res.SigsNeeded = n
	return
}


func NewMultiSigFromScript(p []byte) (*MultiSig, error) {
	r := new(MultiSig)

	var idx, stage int
	for idx < len(p) {
		opcode, pv, n, er := GetOpcode(p[idx:])
		if er != nil {
			return nil, errors.New("NewMultiSigFromScript: " + er.Error())
		}
		idx+= n

		switch stage {
			case 0: // look for OP_FALSE
				if opcode!=0 {
					return nil, errors.New("NewMultiSigFromScript: first opcode must be OP_0")
				}
				stage = 1

			case 1: // look for signatures
				sig, _ := NewSignature(pv)
				if sig!=nil {
					r.Signatures = append(r.Signatures, sig)
					break
				}
				if idx != len(p) {
					return nil, errors.New("NewMultiSigFromScript: unexpected extra data at the end of script")
				}
				p = pv
				idx = 0
				stage = 2

			case 2: // Look for number of required signatures
				if opcode<OP_1 || opcode>OP_16 {
					return nil, errors.New(fmt.Sprint("NewMultiSigFromScript: Unexpected number of required signatures ", opcode-OP_1+1))
				}
				r.SigsNeeded = uint(opcode-OP_1+1)
				stage = 3

			case 3: // Look for public keys
				if len(pv)==33 && (pv[0]|1)==3 || len(pv)==65 && pv[0]==4 {
					r.PublicKeys = append(r.PublicKeys, pv)
					break
				}
				stage = 4
				fallthrough

			case 4: // Look for number of public keys
				if opcode-OP_1+1 != len(r.PublicKeys) {
					return nil, errors.New(fmt.Sprint("NewMultiSigFromScript: Number of public keys mismatch ", opcode-OP_1+1, "/", len(r.PublicKeys)))
				}
				stage = 5

			case 5:
				if opcode==OP_CHECKMULTISIG {
					stage = 6
				} else {
					return nil, errors.New(fmt.Sprintf("NewMultiSigFromScript: Unexpected opcode 0x%02X at the end of script", opcode))
				}
		}
	}

	if stage != 6 {
		return nil, errors.New("NewMultiSigFromScript:  script too short")
	}

	return r, nil
}


func (ms *MultiSig) P2SH() []byte {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(ms.SigsNeeded-1+OP_1))
	for i := range ms.PublicKeys {
		pk := ms.PublicKeys[i]
		WriteVlen(buf, uint32(len(pk)))
		buf.Write(pk)
	}
	buf.WriteByte(byte(len(ms.PublicKeys)-1+OP_1))
	buf.WriteByte(OP_CHECKMULTISIG)
	return buf.Bytes()
}


func (ms *MultiSig) Bytes() []byte {
	buf := new(bytes.Buffer)
	buf.WriteByte(OP_FALSE)
	for i := range ms.Signatures {
		sb := ms.Signatures[i].Bytes()
		WriteVlen(buf, uint32(len(sb)))
		buf.Write(sb)
	}
	p2sh := ms.P2SH()
	WritePutLen(buf, uint32(len(p2sh)))
	buf.Write(p2sh)
	return buf.Bytes()
}


func (ms *MultiSig) PkScript() (pkscr []byte) {
	pkscr = make([]byte, 23)
	pkscr[0] = 0xa9
	pkscr[1] = 20
	RimpHash(ms.P2SH(), pkscr[2:22])
	pkscr[22] = 0x87
	return
}

func (ms *MultiSig) BtcAddr(testnet bool) *BtcAddr {
	var h [20]byte
	RimpHash(ms.P2SH(), h[:])
	return NewAddrFromHash160(h[:], AddrVerScript(testnet))
}