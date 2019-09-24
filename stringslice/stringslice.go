package stringslice

// Index returns the index of item in a given array
func Index(arr []string, item string) int {
	for i := 0; i < len(arr); i++ {
		if item == arr[i] {
			return i
		}
	}
	return -1
}

// Contains returns true if the given array contains given item
func Contains(arr []string, item string) bool {
	return Index(arr, item) >= 0
}
