package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `[
	{
		"id": "zfs_stats_collection_id",
		"listRule": null,
		"viewRule": null,
		"createRule": null,
		"updateRule": null,
		"deleteRule": null,
		"name": "zfs_stats",
		"type": "base",
		"fields": [
			{
				"autogeneratePattern": "[a-z0-9]{15}",
				"hidden": false,
				"id": "text3208210256",
				"max": 15,
				"min": 15,
				"name": "id",
				"pattern": "^[a-z0-9]+$",
				"presentable": false,
				"primaryKey": true,
				"required": true,
				"system": true,
				"type": "text"
			},
			{
				"cascadeDelete": true,
				"collectionId": "2hz5ncl8tizk5nx",
				"hidden": false,
				"id": "zfs_system_relation",
				"maxSelect": 1,
				"minSelect": 0,
				"name": "system",
				"presentable": false,
				"required": true,
				"system": false,
				"type": "relation"
			},
			{
				"hidden": false,
				"id": "zfs_subtype",
				"name": "subtype",
				"presentable": false,
				"required": true,
				"system": false,
				"type": "select",
				"values": ["arc", "pool", "dataset"]
			},
			{
				"hidden": false,
				"id": "zfs_name",
				"name": "name",
				"presentable": false,
				"system": false,
				"type": "text"
			},
			{
				"hidden": false,
				"id": "zfs_stats",
				"maxSize": 2000000,
				"name": "stats",
				"presentable": false,
				"required": true,
				"system": false,
				"type": "json"
			},
			{
				"hidden": false,
				"id": "zfs_type",
				"maxSelect": 1,
				"name": "type",
				"presentable": false,
				"required": true,
				"system": false,
				"type": "select",
				"values": ["1m", "10m", "20m", "120m", "480m"]
			},
			{
				"hidden": false,
				"id": "autodate_zfs_created",
				"name": "created",
				"onCreate": true,
				"onUpdate": false,
				"presentable": false,
				"system": false,
				"type": "autodate"
			},
			{
				"hidden": false,
				"id": "autodate_zfs_updated",
				"name": "updated",
				"onCreate": true,
				"onUpdate": true,
				"presentable": false,
				"system": false,
				"type": "autodate"
			}
		],
		"system": false
	}]`

		err := app.ImportCollectionsByMarshaledJSON([]byte(jsonData), false)
		if err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		return nil
	})
}

