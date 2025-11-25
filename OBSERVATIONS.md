# Performance / usability 

There should be several filter options to filter out versions, e.g.:
- N days old or younger
- at least N days old
- tag patterns

## Delete behavior when deleting single package versions

When an attestation version is deleted from a graph, then ghcrctl triggers a warning (issued by oras or gh api?):
```
Warning: failed to determine attestation type for sha256:18a6ae3b2b10a3f9e649cd8de3b434bfc0daecda644b2ed00bcd6f944d5b1369: failed to fetch manifest: sha256:18a6ae3b2b10a3f9e649cd8de3b434bfc0daecda644b2ed00bcd6f944d5b1369: not found
```
The deleted attestations are still shown, but as unknown:
```
Image Index: sha256:6aea791a2d8d2795505edc2ee029c7d8f2b76cff34912259b55fe0ad94d612c0
  Tags: [newest, classic-pat-test, stable, test-1, v1.0, latest]
  Version ID: 588579607
  │
  ├─ Platform Manifests (references):
  │    ├─ linux/amd64
  │       Digest: sha256:f0406fcc380e...
  │       Size: 669 bytes
  │    └─ linux/arm64
  │       Digest: sha256:3cc4528291f7...
  │       Size: 669 bytes
  │
  └─ Attestations (referrers):
         ├─ unknown
            Digest: sha256:18a6ae3b2b10...
         └─ unknown
            Digest: sha256:cca2d3fbaac0...
```


# Delete behavior of ghcr / github packages

I am doing:

> docker pull ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest
> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .SBOM}}
> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .Provenance}}"

Both, SBOM and Provenance are present.

Then I get the SBOM/Provenance version-ids (two, because there are two arch manifests) using

> ./ghcrctl versions ghcrctl-test-with-sbom

Then I delete these versions:

> ./ghcrctl delete ghcrctl-test-with-sbom <second versionid>
> ./ghcrctl delete ghcrctl-test-with-sbom <first versionid>


I delete the previously pulled image:

> docker rmi ghcr.io/mkoepf/ghcrctl-test-with-sbom

Then, again:

> docker pull ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest

Now, trying to print the attestations results in errors.

> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .SBOM}}"

ERROR: failed to copy: httpReadSeeker: failed open: content at https://ghcr.io/v2/mkoepf/ghcrctl-test-with-sbom/manifests/sha256:c

> docker buildx imagetools inspect ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest --format "{{json .Provenance}}"

ERROR: failed to copy: httpReadSeeker: failed open: content at https://ghcr.io/v2/mkoepf/ghcrctl-test-with-sbom/manifests/sha256:cca2d3fbaac0f656634786844f9e002230d23100b84c4201c9f53c44392130bc not found: not found

Next, I delete the linux/arm64 manifest (whose version-id I get with ghcrctl versions).

Then, I delete the previously pulled image, again:

> docker rmi ghcr.io/mkoepf/ghcrctl-test-with-sbom

When I now try to pull, I get an error:

> docker pull ghcr.io/mkoepf/ghcrctl-test-with-sbom:latest

latest: Pulling from mkoepf/ghcrctl-test-with-sbom
manifest unknown

Note, that this is different from trying to pull an image that never had an linux/arm64 manifest, e.g.:

> docker pull ghcr.io/mkoepf/ghcrctl-test-no-sbom:latest
latest: Pulling from mkoepf/ghcrctl-test-no-sbom
no matching manifest for linux/arm64/v8 in the manifest list entries

## Conclusion:

Delete parts of a graph will be a rare use case, if at all. It breaks the logic
of the image.

In all normal cases, the graph is actually the atom of ghcr operations. This is
how ghcrctl should also operate.