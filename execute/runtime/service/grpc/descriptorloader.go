package grpc

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/ygrebnov/errorc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

type BufDescriptorLoader struct {
	client *http.Client
	token  string
}

func NewBufDescriptorLoader(
	client *http.Client,
	token string,
) *BufDescriptorLoader {
	if client == nil {
		client = http.DefaultClient
	}

	return &BufDescriptorLoader{
		client: client,
		token:  token,
	}
}

func (l *BufDescriptorLoader) Load(
	ctx context.Context,
	bufModule string,
) (*descriptorpb.FileDescriptorSet, error) {
	url, err := descriptorURL(bufModule)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		http.NoBody,
	)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCDescriptor,
			errorc.String(keys.BufModule, bufModule),
			errorc.Error(keys.Cause, err),
		)
	}

	req.Header.Set(string(keys.HeaderAccept), string(keys.MediaTypeOctetStream))

	if l.token != "" {
		req.Header.Set(string(keys.HeaderAuthorization), string(keys.AuthorizationSchemeBearer)+" "+l.token)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCDescriptor,
			errorc.String(keys.BufModule, bufModule),
			errorc.Error(keys.Cause, err),
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4<<10),
		)

		return nil, errorc.With(
			errors.ErrCannotLoadGRPCDescriptor,
			errorc.String(keys.BufModule, bufModule),
			errorc.Int(keys.HTTPStatusCode, resp.StatusCode),
			errorc.String(
				keys.HTTPResponseBody,
				strings.TrimSpace(string(body)),
			),
		)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCDescriptor,
			errorc.String(keys.BufModule, bufModule),
			errorc.String(keys.Operation, "read descriptor response"),
			errorc.Error(keys.Cause, err),
		)
	}

	descriptorSet := &descriptorpb.FileDescriptorSet{}

	if err := proto.Unmarshal(
		data,
		descriptorSet,
	); err != nil {
		return nil, errorc.With(
			errors.ErrCannotLoadGRPCDescriptor,
			errorc.String(keys.BufModule, bufModule),
			errorc.String(keys.Operation, "decode descriptor response"),
			errorc.Error(keys.Cause, err),
		)
	}

	return descriptorSet, nil
}

func descriptorURL(
	bufModule string,
) (string, error) {
	const prefix = "buf.build/"

	if !strings.HasPrefix(bufModule, prefix) {
		return "", errorc.With(
			errors.ErrInvalidBufModule,
			errorc.String(keys.BufModule, bufModule),
		)
	}

	path := strings.TrimPrefix(
		bufModule,
		prefix,
	)

	parts := strings.Split(path, "/")

	if len(parts) != 2 ||
		parts[0] == "" ||
		parts[1] == "" {
		return "", errorc.With(
			errors.ErrInvalidBufModule,
			errorc.String(keys.BufModule, bufModule),
		)
	}

	return "https://buf.build/" +
		path +
		"/descriptor/main", nil
}
