# JSON-RPC Tokenlist Config Workflow 教程

这篇文档说明如何使用：

```text
.github/workflows/jsonrpc-tokenlist-config.yml
```

它是一个手工触发的 GitHub Actions workflow，用来更新 tokenlist 相关配置，并重新生成：

```text
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

## 这个 Workflow 会改什么

这个 workflow 只会修改：

```text
extensions/jsonrpc/config/
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

它不会修改：

```text
blockchains/**
```

适用场景：

- 给一个已经存在的本地资产加或改手工 override
- 手工新增一个最终 tokenlist 条目，或者删除它
- 替换或调整当前 hot 列表

## GitHub 页面操作步骤

1. 打开仓库的 GitHub 页面
2. 进入 `Actions`
3. 打开 `Update JSON-RPC Tokenlist Config`
4. 点击 `Run workflow`
5. 选择 `operation`
6. 在 `payload_json` 中粘贴 JSON
7. 点击运行
8. 等待 workflow 完成
9. 检查最终提交内容：

```text
extensions/jsonrpc/config/
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

## Operation 总览

可选的 `operation` 有：

```text
override_upsert
override_delete
manual_token_upsert
manual_token_delete
hot_replace_current
hot_add_current
hot_remove_current
hot_reset_current
```

每个 operation 会改什么：

- `override_upsert`：更新 `tokenlist-manual-overrides.json`
- `override_delete`：更新 `tokenlist-manual-overrides.json`
- `manual_token_upsert`：更新 `tokenlist-manual-tokens.json`
- `manual_token_delete`：更新 `tokenlist-manual-tokens.json`
- `hot_replace_current`：整体替换 `tokenlist-hot-current.json`
- `hot_add_current`：向 `tokenlist-hot-current.json` 追加并自动去重
- `hot_remove_current`：从 `tokenlist-hot-current.json` 删除指定条目
- `hot_reset_current`：清空 `tokenlist-hot-current.json`

## 通用输入规则

大部分 operation 都支持下面三种 `payload_json` 形式：

1. 单个对象
2. 对象数组
3. 包装对象，例如 `{ "assetOverrides": [...] }` 或 `{ "tokens": [...] }`

通用校验规则：

- `chain` 必须存在于本地 `blockchains/<chain>`
- `manual_token_upsert` 只支持 `kind: "token"`
- 不支持手工 native
- `manual_token_delete` 只读取 `chain` 和 `address`
- `hot_reset_current` 必须把 `payload_json` 留空
- 同一次 manual token payload 里如果有重复的 `chain + address`，会直接失败

## 1. `override_upsert`

当资产已经存在于本地 `blockchains/**`，你只是想覆盖展示名、symbol、market 绑定或额外 tags 时，用这个。

常用字段：

- `chain`
- `address`
- `coingeckoId`
- `displayName`
- `displaySymbol`
- `addTags`
- `note`

单对象示例：

```json
{
  "chain": "solana",
  "address": "So11111111111111111111111111111111111111112",
  "displayName": "Wrapped SOL",
  "displaySymbol": "SOL",
  "addTags": ["wrapped"],
  "note": "用于 app 展示重命名"
}
```

包装数组示例：

```json
{
  "assetOverrides": [
    {
      "chain": "smartchain",
      "address": "0x55d398326f99059fF775485246999027B3197955",
      "displayName": "Tether USD",
      "coingeckoId": "tether"
    },
    {
      "chain": "base",
      "address": "0x833589fCD6EDB6E08f4c7C32D4f71b54bdA02913",
      "displaySymbol": "USDC"
    }
  ]
}
```

行为：

- 如果 `chain + address` 已存在：替换旧的 manual override
- 如果 `chain + address` 不存在：新增一条 manual override

## 2. `override_delete`

用于删除一个或多个 manual override。

这里只有 `chain` 和 `address` 有效。

示例：

```json
{
  "chain": "smartchain",
  "address": "0x55d398326f99059fF775485246999027B3197955"
}
```

数组示例：

```json
[
  {
    "chain": "smartchain",
    "address": "0x55d398326f99059fF775485246999027B3197955"
  },
  {
    "chain": "base",
    "address": "0x833589fCD6EDB6E08f4c7C32D4f71b54bdA02913"
  }
]
```

行为：

- 找到条目：删除
- 没找到：忽略，不报错

## 3. `manual_token_upsert`

当一个 token 不存在于本地 `blockchains/**`，但你仍然希望它出现在最终：

```text
extensions/jsonrpc/data/tokenlist.json
```

时，就用这个。

它会写入：

```text
extensions/jsonrpc/config/tokenlist-manual-tokens.json
```

关键规则：

- 只支持 `kind: "token"`
- `address` 必填
- 这类 token 会追加在自动生成的本地 token 之后
- 如果 `chain + address` 和本地资产冲突，workflow 会失败
- 字段按你提供的内容保留；它本身就是最终 token 形态

最小示例：

```json
{
  "kind": "token",
  "chain": "solana",
  "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL",
  "assetId": "solana:METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL",
  "name": "Meteora",
  "symbol": "MET",
  "decimals": 6,
  "status": "active",
  "hot": true
}
```

完整一点的示例：

```json
{
  "tokens": [
    {
      "kind": "token",
      "chain": "solana",
      "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL",
      "assetId": "solana:METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL",
      "name": "Meteora",
      "symbol": "MET",
      "decimals": 6,
      "status": "active",
      "logoURI": "https://example.com/met.png",
      "logoExists": true,
      "hot": true,
      "tags": ["defi"],
      "links": [
        {
          "name": "website",
          "url": "https://meteora.ag"
        }
      ]
    }
  ]
}
```

行为：

- 如果 `chain + address` 已存在于 `tokenlist-manual-tokens.json`：替换原条目
- 如果不存在：新增一条

## 4. `manual_token_delete`

用于删除之前通过 `manual_token_upsert` 添加的 token。

这里只有 `chain` 和 `address` 有效。

示例：

```json
{
  "chain": "solana",
  "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL"
}
```

行为：

- 找到 manual token：删除
- 没找到 manual token：忽略，不报错

## 5. `hot_replace_current`

当你要整体替换“当前周期 hot 列表”时，用这个。

如果你已经准备好了完整的当前 hot 快照，这是最合适的操作。

示例：

```json
{
  "tokens": [
    {
      "chain": "solana",
      "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL"
    },
    {
      "chain": "smartchain",
      "address": "0x55d398326f99059fF775485246999027B3197955"
    }
  ]
}
```

行为：

- 旧的 current hot 列表会被整体替换
- 重复项会自动去重

## 6. `hot_add_current`

当你只想往 current hot 列表里追加几个条目，而不是整体替换时，用这个。

示例：

```json
[
  {
    "chain": "solana",
    "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL"
  },
  {
    "chain": "base",
    "address": "0x833589fCD6EDB6E08f4c7C32D4f71b54bdA02913"
  }
]
```

行为：

- 新条目会被追加进去
- 原有条目会保留
- 重复项会自动去重

## 7. `hot_remove_current`

当你想从 current hot 列表里删掉某几个指定条目时，用这个。

示例：

```json
{
  "chain": "solana",
  "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL"
}
```

行为：

- 找到条目：删除
- 没找到：忽略，不报错

## 8. `hot_reset_current`

当你想把 current hot 列表整个清空时，用这个。

这个操作要求：

- `operation = hot_reset_current`
- `payload_json = ""`

也就是不要传任何 JSON。

执行后：

- `tokenlist-hot-current.json` 会变成：

```json
{
  "tokens": []
}
```

## 常见日常用法

给一个本地已有 token 改展示信息：

1. 运行 `override_upsert`
2. 填 `chain + address`
3. 设置 `displayName` 和/或 `displaySymbol`

把一个本地不存在的 token 直接塞进最终 tokenlist：

1. 运行 `manual_token_upsert`
2. 粘贴一个 `kind: "token"` 的最终 token 对象
3. 等待 `tokenlist.json` 重新生成

删除一个之前手工新增的 token：

1. 运行 `manual_token_delete`
2. 只粘贴 `chain` 和 `address`

整体刷新当前 hot 集：

1. 准备完整的 current hot 快照
2. 运行 `hot_replace_current`
3. 粘贴整份快照

临时补一个 hot token：

1. 运行 `hot_add_current`
2. 粘贴一个 token 引用对象

## 常见报错

`manual token uses unknown chain`

- 说明 `chain` 在本地 `blockchains/<chain>` 下不存在

`manual token ... missing address`

- 说明 `manual_token_upsert` 缺少合约地址

`unsupported kind "native"; only kind=token is allowed`

- 说明手工 native 不支持，只能加手工 token

`conflicts with a local asset`

- 说明这个 token 已经存在于本地 `blockchains/**`
- 这种情况应该用 `override_upsert`，不是 `manual_token_upsert`

`duplicate manual token key`

- 说明同一次 payload 里重复出现了相同的 `chain + address`

`hot token uses unknown chain`

- 修正 hot payload 里的 `chain`

`hot_reset_current does not accept payload_json`

- `hot_reset_current` 时把 `payload_json` 留空

## 快速判断

如果资产本地已经存在，用 `override_*`。

如果资产本地不存在，但必须出现在最终 app tokenlist，用 `manual_token_*`。

如果你只是想改当前 hot 状态，用 `hot_*`。
