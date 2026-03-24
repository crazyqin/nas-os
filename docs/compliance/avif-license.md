# AVIF格式许可审查报告

**审查日期**: 2026年3月25日  
**审查部门**: 刑部  
**项目**: nas-os AVIF格式支持功能

---

## 1. 概述

本报告针对nas-os项目计划引入的AVIF图像格式支持功能进行许可合规审查，分析libavif库的许可证兼容性及相关专利风险。

---

## 2. libavif许可证分析

### 2.1 许可证类型

libavif采用 **BSD 2-Clause许可证**（简化版BSD许可证）。

### 2.2 BSD许可证特点

| 特性 | 说明 |
|------|------|
| **商业使用** | ✅ 允许商业用途 |
| **修改分发** | ✅ 允许修改和再分发 |
| **闭源使用** | ✅ 允许闭源项目中使用 |
| **专利授权** | ❌ 不包含明确的专利授权条款 |
| **担保免责** | ✅ 包含标准免责条款 |

### 2.3 许可证兼容性

BSD许可证属于宽松型许可证（Permissive License），与以下许可证兼容：

| 许可证 | 兼容性 |
|--------|--------|
| MIT | ✅ 完全兼容 |
| Apache 2.0 | ✅ 完全兼容 |
| GPL v2/v3 | ✅ 兼容（BSD代码可作为GPL项目的一部分） |
| LGPL | ✅ 兼容 |
| 专有软件 | ✅ 允许集成到闭源项目 |

**结论**: BSD许可证对本项目无合规障碍，可直接使用。

---

## 3. AVIF/AV1专利风险评估

### 3.1 AVIF与AV1的关系

- AVIF（AV1 Image File Format）基于AV1视频编解码器
- AV1由AOMedia（开放媒体联盟）开发
- AVIF于2019年标准化，使用AV1帧内编码压缩图像

### 3.2 AOMedia专利政策

**AOMedia的专利许可模式**:

1. **RF Licensing（Royalty-Free）**
   - AOMedia成员承诺对AV1相关专利提供免版税许可
   - 成员需遵守AOMedia专利许可协议

2. **专利池保护**
   - 主要成员包括：Google、Apple、Microsoft、Netflix、Amazon、Meta、Intel、NVIDIA、Cisco等
   - 成员间交叉许可有效覆盖大部分核心专利

### 3.3 与HEVC的对比

| 对比项 | AV1/AVIF | HEVC/H.265 |
|--------|----------|------------|
| 专利池 | 单一（AOMedia） | 多个（MPEG LA、HEVC Advance、Velos Media） |
| 版税 | 免费 | 需支付（编码/解码/内容） |
| 专利风险 | 低 | 高（专利陷阱） |
| 企业支持 | 广泛 | 分裂 |

### 3.4 潜在风险点

| 风险类型 | 风险等级 | 说明 |
|----------|----------|------|
| SEPs（标准必要专利） | 🟡 中 | 非AOMedia成员可能持有AV1相关专利 |
| 专利声明 | 🟢 低 | AOMedia要求成员披露相关专利 |
| 第三方专利 | 🟡 中 | 存在非成员专利风险（但实践中较少） |
| 专利诉讼历史 | 🟢 低 | AV1至今无重大专利诉讼 |

### 3.5 风险缓解建议

1. **持续关注AOMedia公告** - 关注专利许可政策更新
2. **版本更新跟踪** - 使用官方稳定版本，及时更新
3. **文档留存** - 保留许可文件和版权声明
4. **法律咨询** - 如遇专利声明，及时咨询专业法律顾问

---

## 4. 依赖库许可证链

libavif的可选依赖项许可证情况：

| 依赖库 | 用途 | 许可证 | 兼容性 |
|--------|------|--------|--------|
| libaom | AV1编解码 | BSD+AOM许可证 | ✅ 兼容 |
| dav1d | AV1解码 | BSD | ✅ 兼容 |
| rav1e | AV1编码 | BSD | ✅ 兼容 |
| SVT-AV1 | AV1编码 | BSD-3-Clause | ✅ 兼容 |
| libyuv | 色彩空间转换 | BSD-3-Clause | ✅ 兼容 |

**结论**: 所有推荐依赖项均为宽松许可证，无许可证冲突。

---

## 5. 合规要求

### 5.1 必须履行的义务

1. **版权声明保留**
   - 在分发时保留libavif的版权声明
   - 保留BSD许可证全文

2. **免责条款保留**
   - 保留"AS IS"免责声明

### 5.2 无需履行的义务

- ❌ 无需开源本项目代码
- ❌ 无需支付版税
- ❌ 无需披露衍生作品源码

---

## 6. 使用建议

### 6.1 推荐做法

| 建议 | 说明 |
|------|------|
| ✅ 使用官方发布版本 | 避免使用不稳定分支 |
| ✅ 记录依赖版本 | 便于追溯和更新 |
| ✅ 包含LICENSE文件 | 在项目中包含libavif的BSD许可证 |
| ✅ 定期更新 | 关注安全更新和版本发布 |
| ✅ 选择合适的编解码后端 | 推荐libaom或dav1d |

### 6.2 不推荐做法

| 行为 | 风险 |
|------|------|
| ❌ 移除版权声明 | 违反许可证 |
| ❌ 使用非官方分支 | 可能包含不兼容代码 |
| ❌ 忽略安全更新 | 安全风险 |

---

## 7. 结论

### 7.1 许可证合规性

**结论**: ✅ **合规通过**

libavif采用BSD许可证，与本项目无许可证冲突，可自由使用、修改和分发。

### 7.2 专利风险评估

**结论**: ✅ **低风险**

AV1/AVIF由AOMedia主导，采用免版税专利许可模式，专利风险远低于HEVC等专利编码格式。

### 7.3 最终建议

**建议采用AVIF格式**，理由如下：

1. 许可证合规无障碍
2. 专利风险可控且较低
3. 技术先进（压缩效率高）
4. 行业趋势支持（主流浏览器已支持）
5. 免版税降低成本

---

## 附录A: libavif许可证全文

```
Copyright 2019 Joe Drago. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

---

## 附录B: 参考链接

- libavif GitHub: https://github.com/AOMediaCodec/libavif
- AOMedia官网: https://aomedia.org/
- AV1规范: https://aomedia.org/specifications/av1/
- AOMedia成员列表: https://aomedia.org/membership/members

---

*本报告由刑部编制，仅供参考，不构成法律意见。如有具体法律问题，请咨询专业律师。*