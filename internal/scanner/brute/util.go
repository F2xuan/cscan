package brute

// appendInt32 appends a little-endian int32 to buf
func appendInt32(buf []byte, v int32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// appendInt64 appends a little-endian int64 to buf
func appendInt64(buf []byte, v int64) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

// appendUint16 appends a little-endian uint16 to buf
func appendUint16(buf []byte, v uint16) []byte {
	return append(buf, byte(v), byte(v>>8))
}

// appendUint32 appends a little-endian uint32 to buf
func appendUint32(buf []byte, v uint32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
