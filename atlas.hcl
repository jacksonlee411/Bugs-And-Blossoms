// Atlas configuration (SSOT) for module-level schema/migrations.
// Convention: each module owns its own migrations directory and goose version table.

env "iam_dev" {
  src = "file://modules/iam/infrastructure/persistence/schema"
  migration {
    dir    = "file://migrations/iam"
    format = "goose"
  }
}

env "iam_ci" {
  src = "file://modules/iam/infrastructure/persistence/schema"
  migration {
    dir    = "file://migrations/iam"
    format = "goose"
  }
}

env "orgunit_dev" {
  src = "file://modules/orgunit/infrastructure/persistence/schema"
  migration {
    dir    = "file://migrations/orgunit"
    format = "goose"
  }
}

env "orgunit_ci" {
  src = "file://modules/orgunit/infrastructure/persistence/schema"
  migration {
    dir    = "file://migrations/orgunit"
    format = "goose"
  }
}
