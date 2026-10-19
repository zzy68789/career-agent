import type { Degree } from "./types.ts"

export type Sample = {
  id: string
  label: string
  degree: Degree
  resume: string
}

export const samples: Sample[] = [
  {
    id: "cs-bachelor",
    label: "计算机本科",
    degree: "bachelor",
    resume: `张同学，郑州大学计算机与人工智能学院软件工程专业，2026 届本科。
熟悉 Go 语言，课程中使用 MySQL 与 Redis 完成宿舍报修系统，负责接口和简单缓存。
曾在郑州一家软件公司实习三个月，使用 Git 做版本管理，在 Linux 上部署后端服务。
了解 Java，项目主力语言是 Go。没有前端框架经验。
求职意向：郑州，后端开发。`,
  },
  {
    id: "cs-master",
    label: "计算机硕士",
    degree: "master",
    resume: `李同学，郑州大学计算机与人工智能学院计算机科学与技术专业，2026 届硕士。
研究方向为自然语言处理，熟悉 Python、PyTorch，完成过简历与岗位文本匹配实验。
一篇中文核心论文在投。本科阶段做过数据处理，SQL 使用一般。
求职意向：算法工程师，城市不限。`,
  },
  {
    id: "ee-bachelor",
    label: "电气本科",
    degree: "bachelor",
    resume: `王同学，郑州大学电气与信息工程学院电气工程及其自动化专业，2026 届本科。
课程设计完成小型供配电方案，能使用 AutoCAD 绘制电气图纸，了解 PLC。
参加过单片机实验，用 C 语言读取传感器并控制流水灯。
求职意向：郑州，电气设计或嵌入式软件。`,
  },
]
