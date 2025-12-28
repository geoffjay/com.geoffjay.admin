package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		// Check if users collection already exists
		_, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err == nil {
			return nil // Collection already exists
		}

		collection := core.NewAuthCollection("users")
		collection.Id = "_pb_users_auth_"

		collection.Fields.Add(&core.TextField{
			Name:     "name",
			Required: false,
			Max:      255,
		})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			return nil // Collection doesn't exist, nothing to do
		}

		return app.Delete(collection)
	})
}
