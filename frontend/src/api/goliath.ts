import { parseJson } from '../utils/helpers';

/** VersionData describes metadata about the backend version. */
export type VersionData = {
  build_timestamp: string;
  build_hash: string;
  /** The schema version, ready to show: the database's too if it differs. */
  schema: string;
};

/** describeSchema says which schema version the backend is running on. */
function describeSchema(binary?: string, database?: string): string {
  if (!binary) {
    return '<unknown>';
  }
  if (!database || database === binary) {
    return `v${binary}`;
  }
  return `v${binary} (database v${database})`;
}

/** GetVersion returns metadata about the Goliath backend. */
export async function GetVersion(): Promise<VersionData> {
  // version matches the response from a /version API call.
  interface version {
    build_timestamp: string;
    build_hash: string;
    schema_version?: string;
    db_schema_version?: string;
  }

  return await fetch('/version', {
    credentials: 'include',
  })
    .then((result) => result.text())
    .then((result) => parseJson(result))
    .then((body: version): VersionData => {
      return {
        build_timestamp: body.build_timestamp,
        build_hash: body.build_hash,
        schema: describeSchema(body.schema_version, body.db_schema_version),
      };
    })
    .catch((e): VersionData => {
      console.log(
        'Error while fetching version, returning unknown version info: ' + e
      );
      return {
        build_timestamp: '<unknown>',
        build_hash: '<unknown>',
        schema: '<unknown>',
      };
    });
}
