FROM alpine:3.22
ARG GITHUB_REPOSITORY
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
LABEL org.opencontainers.image.description="Test image with SBOM and provenance (multiarch)"
LABEL test.image.type="with-sbom-with-provenance-multiarch"
RUN echo "Test image 2 - with SBOM and provenance" > /test.txt
HEALTHCHECK --interval=30s --timeout=3s CMD cat /test.txt || exit 1
USER nobody
CMD ["cat", "/test.txt"]
