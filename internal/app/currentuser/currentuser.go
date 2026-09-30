package currentuser

// ID is fixed for lab 3. Authentication will be implemented in lab 4.
const ID uint = 1

// Get returns the same current user for every request (singleton).
func Get() uint {
	return ID
}
