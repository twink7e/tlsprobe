package creator

type Creator interface {
	Describe() string
}

type GetCreator interface {
	GetCreator() Creator
}
