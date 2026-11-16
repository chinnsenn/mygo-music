# internal/lyrics/
> L2 | 父级: ../../AGENTS.md

歌词解析只依赖 music 数据契约；source 子包负责目录检索，播放器负责决定何时补全并发布解析结果。

成员清单
lyrics.go: 将音源逐字行或 LRC 与翻译转换为统一时间线，供播放器和各歌词视图消费。
lyrics_test.go: 验证时间戳、翻译关联、逐字时序、语种与间奏识别。
source/: 按元数据匹配网易、QQ、酷狗的歌词目录，管理有界查询与缓存；见 source/AGENTS.md。

[PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md
