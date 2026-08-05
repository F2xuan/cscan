package brute

// appendUint16 appends a little-endian uint16 to buf
func appendUint16(buf []byte, v uint16) []byte {
	return append(buf, byte(v), byte(v>>8))
}

// appendUint32 appends a little-endian uint32 to buf
func appendUint32(buf []byte, v uint32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
