package consul

import (
	"context"
	"fmt"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
)

const (
	SecretTokenType = "token"
)

func secretToken(b *backend) *framework.Secret {
	return &framework.Secret{
		Type: SecretTokenType,
		Fields: map[string]*framework.FieldSchema{
			"token": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Request token",
			},
		},

		Renew:  b.secretTokenRenew,
		Revoke: secretTokenRevoke,
	}
}

func (b *backend) secretTokenRenew(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleRaw, ok := req.Secret.InternalData["role"]
	if !ok || roleRaw == nil {
		return framework.LeaseExtend(0, 0, b.System())(ctx, req, d)
	}

	role, ok := roleRaw.(string)
	if !ok {
		return framework.LeaseExtend(0, 0, b.System())(ctx, req, d)
	}

	entry, err := req.Storage.Get("policy/" + role)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role: %s", err)
	}
	if entry == nil {
		return logical.ErrorResponse(fmt.Sprintf("issuing role %q not found", role)), nil
	}

	var result roleConfig
	if err := entry.DecodeJSON(&result); err != nil {
		return nil, err
	}

	return framework.LeaseExtend(result.Lease, 0, b.System())(ctx, req, d)
}

func secretTokenRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	c, userErr, intErr := client(req.Storage)
	if intErr != nil {
		return nil, intErr
	}
	if userErr != nil {
		// Returning logical.ErrorResponse from revocation function is risky
		return nil, userErr
	}

	tokenRaw, ok := req.Secret.InternalData["token"]
	if !ok {
		// We return nil here because this is a pre-0.5.3 problem and there is
		// nothing we can do about it. We already can't revoke the lease
		// properly if it has been renewed and this is documented pre-0.5.3
		// behavior with a security bulletin about it.
		return nil, nil
	}

	_, err := c.ACL().Destroy(tokenRaw.(string), nil)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
