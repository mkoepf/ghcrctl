FROM gcr.io/distroless/static:nonroot
ARG GITHUB_REPOSITORY
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
LABEL org.opencontainers.image.description="Test image with SBOM but no provenance"
LABEL test.image.type="with-sbom-no-provenance"
USER nobody
