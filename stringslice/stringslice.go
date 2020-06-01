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

// Remove removes an item from array if it's in there
func Remove(arr *[]string, item string) {
	idx := Index(*arr, item)
	if idx >= 0 {
		*arr = append((*arr)[0:idx], (*arr)[idx+1:len(*arr)]...)
	}
}
