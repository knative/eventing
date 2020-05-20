# Ensure for Go

A simple ensure package for Golang

## Use case

Writing a Go code makes a lot of repetition, regarding error checking. That's 
especially true for end user code like e2e tests when we expect that something 
will work. In other cases that's a bug and should code should panic then.

## Usage

Instead of writing:

```go
func DeployAllComponents() error {
  alpha, err := deployAlpha()
  if err != nil {
    return errors.WithMessage(err, "unexpected error")
  }
  beta, err := deployBeta(alpha)
  if err != nil {
    return errors.WithMessage(err, "unexpected error")
  }
  _, err := deployGamma(beta)
  if err != nil {
    return errors.WithMessage(err, "unexpected error")
  }
  return nil
}

// execution isn't simple
err = DeployAllComponents()
if err != nil {
  panic(err)
}
```

with this PR I can write it like:

```go
func DeployAllComponents() {
  alpha, err := deployAlpha()
  ensure.NoError(err)
  beta, err := deployBeta(alpha)
  ensure.NoError(err)
  _, err := deployGamma(beta)
  ensure.NoError(err)
}

// execution is simple
DeployAllComponents()
```

Above is much more readable and pleasant to see.
