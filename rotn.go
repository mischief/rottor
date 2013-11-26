package rottor

func RotNEnc(n int) Filter {
	return func(b byte) byte {
		return byte((int(b) + n) % 256)
	}
}

func RotNDec(n int) Filter {
	return func(b byte) byte {
		return byte((int(b) - n) % 256)
	}
}
