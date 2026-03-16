// Package auth defines small authentication interfaces shared by the Codex runtime.
package auth

// TokenStorage defines the interface for storing authentication tokens.
// Implementations of this interface should provide methods to persist
// authentication tokens to a file system location.
type TokenStorage interface {
	// SaveTokenToFile persists authentication tokens to the specified file path.
	//
	// Parameters:
	//   - authFilePath: The file path where the authentication tokens should be saved
	//
	// Returns:
	//   - error: An error if the save operation fails, nil otherwise
	SaveTokenToFile(authFilePath string) error
}
