package social

// ReaderInterface permite mockear Reader en tests de comandos.
type ReaderInterface interface {
	FetchSocialPosts(input InputFetch) ([]OutputPost, error)
}
