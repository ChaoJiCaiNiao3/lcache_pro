package lcache_pro

// ByteView 只读的字节视图，用于缓存数据
type ByteView []byte

func (b ByteView) Len() int {
	return len(b)
}

func (b ByteView) ByteSLice() []byte {
	return cloneBytes(b)
}

func (b ByteView) String() string {
	return string(b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
