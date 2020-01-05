package vfs

func AlignNumber(number, multiplier int) int {
	return number + (multiplier - (number % multiplier))
}
