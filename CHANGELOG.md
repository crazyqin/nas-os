# Changelog

All notable changes to NAS-OS will be documented in this file.

## [v2.253.289] - 2026-03-24

### Added
- **OpenAI Compatible API** (`internal/ai/openai_compat.go`): Universal AI service integration
  - Support for any OpenAI-compatible API endpoint
  - Streaming chat support
  - Full OpenAI API format compatibility
  - Inspired by Synology AI Console

- **China Cloud Providers** (`internal/cloudsync/provider_china.go`): Native support for Chinese cloud storage
  - Aliyun OSS integration
  - Tencent COS integration
  - Baidu Object Storage
  - Inspired by 飞牛fnOS 网盘挂载

- **AI Service Test Coverage** (`internal/ai/*_test.go`): Comprehensive test suite
  - Service layer tests
  - OpenAI compatibility tests
  - De-identification tests

### Documentation
- Competitor analysis report (docs/COMPETITOR_ANALYSIS.md)
- Security audit report (SECURITY_AUDIT_2026-03-24_0936.md)
- User guide updates (docs/USER_GUIDE.md)

### Changed
- Version bump: v2.253.288 → v2.253.289
- AI service enhancements with better error handling

### Fixed
- gofmt and revive linter errors in ai/service.go
- Removed conflicting tiering.go file

### Competitor Analysis
- Studied Synology DSM 7.3: OpenAI-compatible API, data de-identification
- Studied 飞牛fnOS: Native cloud drive mounting for Chinese providers

---

## [v2.253.288] - 2026-03-24

### Added
- **Cloud Mount** (`internal/cloudsync`): Multi-cloud storage mounting
  - Mount Aliyun OSS, Tencent COS, AWS S3, Google Drive, OneDrive as local directories
  - Transparent read/write access
  - Inspired by 飞牛fnOS 网盘挂载

- **AI De-identification** (`internal/ai`): Privacy-first AI integration
  - PII detection: email, phone, ID card, credit card, IP address
  - Multiple AI providers: OpenAI, Google, Azure, Baidu, local LLM
  - Streaming chat support
  - Inspired by Synology AI Console

- **Intelligent Tiering** (`internal/tiering`): Automatic data tiering
  - Hot/warm/cold tier management
  - Access pattern tracking and scoring
  - SSD cache acceleration
  - Cloud archive tier

### Changed
- Version bump: v2.253.287 → v2.253.288
- Documentation updated with new features

### Competitor Analysis
- Studied Synology DSM 7.3 features (Tiering, AI Console, Drive enhancements)
- Studied 飞牛fnOS features (网盘挂载, 智能相册, 影视中心)

---

## [v2.253.287] - 2026-03-24

### Added
- **Tiering Module** (`internal/tiering`): Intelligent data tiering system
  - Automatic hot/warm/cold tier management
  - Access pattern tracking and scoring
  - Configurable promotion/demotion policies
  - Background tiering process
  - Inspired by Synology Tiering

- **AI Service Module** (`internal/ai`): Multi-provider AI integration
  - Support for OpenAI, Google, Azure, Baidu, and local LLMs
  - PII de-identification (email, phone, ID card, credit card, IP)
  - Privacy-first design
  - Streaming chat support
  - Inspired by Synology AI Console

### Changed
- Version bump: v2.253.286 → v2.253.287

### Competitor Analysis
- Studied Synology DSM 7.3 features (Tiering, AI Console, Drive enhancements)
- Studied 飞牛fnOS features (网盘挂载, 智能相册, 影视中心)

---

## [v2.253.286] - 2026-03-24

### Changed
- Version bump for development round 36

---

## [v2.253.285] - 2026-03-23

### Changed
- Six-department collaborative development round 35

---

## Version History

| Version | Date | Key Features |
|---------|------|--------------|
| v2.253.289 | 2026-03-24 | OpenAI Compatible API, China Cloud Providers, AI Tests |
| v2.253.288 | 2026-03-24 | Cloud Mount, AI De-identification, Intelligent Tiering |
| v2.253.287 | 2026-03-24 | Tiering, AI Service modules |
| v2.253.286 | 2026-03-24 | Development round 36 |
| v2.253.285 | 2026-03-23 | Development round 35 |