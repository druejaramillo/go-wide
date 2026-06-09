package ops

import "context"

func Run(ctx context.Context, op string, fn func(context.Context) error) error {
	ctx, err := Start(ctx, op)
	if err != nil {
		return err
	}
	defer End(ctx)

	err = fn(ctx)
	if err != nil {
		Error(ctx, err)
		return err
	}

	return nil
}
