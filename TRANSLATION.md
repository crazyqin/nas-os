# NAS-OS 翻译指南 / Translation Guide

[中文](#中文) | [English](#english)

---

## 中文

### 概述

本文档描述了 NAS-OS 项目的翻译流程和规范，旨在帮助贡献者参与多语言支持工作。

### 支持的语言

| 语言 | 代码 | 状态 |
|------|------|------|
| 简体中文 | zh-CN | ✅ 完整 |
| English | en-US | ✅ 完整 |
| 日本語 | ja-JP | ✅ 完整 |
| 한국어 | ko-KR | ✅ 完整 |

### 翻译文件位置

```
webui/i18n/
├── zh-CN.json    # 简体中文
├── en-US.json    # English
├── ja-JP.json    # 日本語
└── ko-KR.json    # 한국어
```

### 翻译规范

#### 1. JSON 格式

所有翻译文件使用 JSON 格式，按键层级组织：

```json
{
  "namespace": {
    "key": "翻译文本"
  }
}
```

#### 2. 命名规范

- **键名**：使用驼峰命名法 (camelCase)
- **命名空间**：按功能模块划分
- **描述性**：键名应能描述用途

#### 3. 文本规范

- 保持简洁明了
- 使用用户友好的语言
- 避免技术术语（除非必要）
- 保持一致的风格

#### 4. 特殊字符

- 使用 Unicode 转义非 ASCII 字符
- 保留占位符格式 `{variable}`

### 翻译流程

#### 添加新语言

1. 复制 `en-US.json` 为新语言文件（如 `fr-FR.json`）
2. 翻译所有文本值
3. 在 WebUI 中注册新语言
4. 提交 Pull Request

#### 更新现有翻译

1. 找到对应的 JSON 文件
2. 更新需要修改的文本
3. 确保格式正确
4. 提交 Pull Request

#### 添加新翻译键

1. 在所有语言文件中添加新键
2. 确保键名一致
3. 提供所有语言的翻译
4. 更新相关文档

### 质量检查清单

- [ ] JSON 格式正确
- [ ] 所有键都已翻译
- [ ] 无遗漏的占位符
- [ ] 术语翻译一致
- [ ] 文化适应性良好

### 常见问题

**Q: 如何处理未翻译的键？**
A: 系统会回退到默认语言（en-US）。

**Q: 可以使用机器翻译吗？**
A: 可以作为起点，但建议人工审核后再提交。

**Q: 如何报告翻译错误？**
A: 请在 GitHub Issues 中报告，标签为 `translation`。

---

## English

### Overview

This document describes the translation workflow and guidelines for the NAS-OS project, helping contributors participate in multilingual support.

### Supported Languages

| Language | Code | Status |
|----------|------|--------|
| Simplified Chinese | zh-CN | ✅ Complete |
| English | en-US | ✅ Complete |
| Japanese | ja-JP | ✅ Complete |
| Korean | ko-KR | ✅ Complete |

### Translation File Location

```
webui/i18n/
├── zh-CN.json    # Simplified Chinese
├── en-US.json    # English
├── ja-JP.json    # Japanese
└── ko-KR.json    # Korean
```

### Translation Guidelines

#### 1. JSON Format

All translation files use JSON format, organized by key hierarchy:

```json
{
  "namespace": {
    "key": "Translated text"
  }
}
```

#### 2. Naming Conventions

- **Keys**: Use camelCase
- **Namespaces**: Organized by functional modules
- **Descriptive**: Keys should describe their purpose

#### 3. Text Guidelines

- Keep text concise and clear
- Use user-friendly language
- Avoid technical jargon (unless necessary)
- Maintain consistent style

#### 4. Special Characters

- Use Unicode escape for non-ASCII characters
- Preserve placeholder format `{variable}`

### Translation Workflow

#### Adding a New Language

1. Copy `en-US.json` to new language file (e.g., `fr-FR.json`)
2. Translate all text values
3. Register new language in WebUI
4. Submit Pull Request

#### Updating Existing Translations

1. Find the corresponding JSON file
2. Update the text that needs modification
3. Ensure correct format
4. Submit Pull Request

#### Adding New Translation Keys

1. Add new keys in all language files
2. Ensure key names are consistent
3. Provide translations for all languages
4. Update related documentation

### Quality Checklist

- [ ] JSON format is correct
- [ ] All keys are translated
- [ ] No missing placeholders
- [ ] Consistent terminology
- [ ] Good cultural adaptation

### FAQ

**Q: How are untranslated keys handled?**
A: The system falls back to the default language (en-US).

**Q: Can I use machine translation?**
A: Yes, as a starting point, but human review is recommended before submission.

**Q: How do I report translation errors?**
A: Please report in GitHub Issues with the `translation` label.

---

## 贡献者 / Contributors

感谢所有翻译贡献者！

Thank you to all translation contributors!

---

*最后更新 / Last Updated: 2026-03-15*