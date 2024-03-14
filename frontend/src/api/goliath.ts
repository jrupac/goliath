import {parseJson} from "../utils/helpers";
import {VersionData} from "../utils/types";

// version matches the response from a /version API call.
interface version {
  build_timestamp: string;
  build_hash: string;
}

/** GetVersion returns metadata about the Goliath backend. */
export async function GetVersion(): Promise<VersionData> {
  return await fetch('/version', {
    credentials: 'include'
  }).then((result) => result.text())
    .then((result) => parseJson(result))
    .then((body: version): VersionData => {
      return {
        build_timestamp: body.build_timestamp, build_hash: body.build_hash
      };
    }).catch((e): VersionData => {
      console.log(e);
      return {build_timestamp: "<unknown>", build_hash: "<unknown>"};
    });
}