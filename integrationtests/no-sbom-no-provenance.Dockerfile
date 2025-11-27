FROM gcr.io/distroless/static:nonroot
ARG GITHUB_REPOSITORY
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
LABEL org.opencontainers.image.description="Test image without SBOM or provenance (multiarch)"
LABEL test.image.type="no-sbom-no-provenance-multiarch"
USER nobody
