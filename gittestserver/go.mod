module github.com/fluxcd/pkg/gittestserver

go 1.14

// TODO(hidde): drop when PR is accepted:
//  https://github.com/sosedoff/gitkit/pull/21
replace github.com/sosedoff/gitkit => github.com/hiddeco/gitkit v0.2.1-0.20200422093229-4355fec70348

require (
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/sosedoff/gitkit v0.2.0
	github.com/stretchr/testify v1.6.1 // indirect
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de // indirect
)
