<template>
  <div class="traffic-chart">
    <v-chart :option="chartOption" autoresize style="height: 350px; width: 100%" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { TrafficEntry } from '../types'

use([LineChart, TitleComponent, TooltipComponent, LegendComponent, GridComponent, CanvasRenderer])

const props = defineProps<{
  title?: string
  data: TrafficEntry[]
}>()

const chartOption = computed(() => {
  const times = props.data.map((d) => {
    const date = new Date(d.time)
    return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
  })
  const uploads = props.data.map((d) => (d.upload / 1024 / 1024).toFixed(2))
  const downloads = props.data.map((d) => (d.download / 1024 / 1024).toFixed(2))

  return {
    title: {
      text: props.title || '流量趋势',
      left: 'center',
      textStyle: { fontSize: 14 },
    },
    tooltip: {
      trigger: 'axis',
      formatter(params: unknown[]) {
        const items = params as { axisValue: string; seriesName: string; value: string }[]
        let str = `${items[0]?.axisValue || ''}<br/>`
        items.forEach((item) => {
          str += `${item.seriesName}: ${item.value} MB<br/>`
        })
        return str
      },
    },
    legend: {
      data: ['上传', '下载'],
      bottom: 0,
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '12%',
      top: '15%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: times,
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        formatter: '{value} MB',
      },
    },
    series: [
      {
        name: '上传',
        type: 'line',
        smooth: true,
        data: uploads,
        areaStyle: { opacity: 0.3 },
        itemStyle: { color: '#409eff' },
      },
      {
        name: '下载',
        type: 'line',
        smooth: true,
        data: downloads,
        areaStyle: { opacity: 0.3 },
        itemStyle: { color: '#67c23a' },
      },
    ],
  }
})
</script>
