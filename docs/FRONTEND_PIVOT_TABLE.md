# Vue.js Pivot Table Options

This document provides a comprehensive overview of available pivot table solutions for Vue.js applications, helping you choose the best option for your specific needs.

## Table of Contents

1. [Vue 3 Options (Recommended)](#vue-3-options-recommended)
2. [Vue 2 Options (Legacy)](#vue-2-options-legacy)
3. [Commercial Options](#commercial-options)
4. [Comparison Table](#comparison-table)
5. [Implementation Examples](#implementation-examples)
6. [Recommendations](#recommendations)

## Vue 3 Options (Recommended)

### 1. vue-pivottable (Vue 3 Version)

**Status**: Actively maintained (latest release: v1.2.2, Aug 2025)  
**Repository**: https://github.com/vue-pivottable/vue3-pivottable  
**Installation**: `npm install vue-pivottable`

#### Features
- Built with Vue 3 Composition API
- TypeScript support
- Multiple aggregators and renderers
- Interactive drag-and-drop field configuration
- Customizable renderers and styles
- Plotly integration for charts
- Large dataset support with Object.freeze optimization

#### Usage Example
```vue
<template>
  <VuePivottableUi
    :data="pivotData"
    :rows="['category', 'subcategory']"
    :cols="['date']"
    :aggregator-name="'Sum'"
    :val="'amount'"
    renderer-name="Table"
  />
</template>

<script setup>
import { VuePivottableUi } from 'vue-pivottable'
import 'vue-pivottable/dist/vue-pivottable.css'

const pivotData = ref([
  { category: 'Electronics', subcategory: 'Laptops', date: '2024-01', amount: 1500 },
  { category: 'Electronics', subcategory: 'Phones', date: '2024-01', amount: 800 },
  { category: 'Clothing', subcategory: 'Shirts', date: '2024-01', amount: 50 }
])
</script>
```

#### Advanced Configuration
```vue
<template>
  <VuePivottableUi
    :data="pivotData"
    :rows="['category']"
    :cols="['date']"
    :aggregator-name="'Count'"
    renderer-name="Grouped Column Chart"
    :renderers="customRenderers"
    :unused-orientation-cutoff="Infinity"
  />
</template>

<script setup>
import { VuePivottableUi } from 'vue-pivottable'
import PlotlyRenderer from '@vue-pivottable/plotly-renderer'

const customRenderers = computed(() => ({
  'Grouped Column Chart': PlotlyRenderer['Grouped Column Chart'],
  'Stacked Column Chart': PlotlyRenderer['Stacked Column Chart'],
  'Line Chart': PlotlyRenderer['Line Chart']
}))
</script>
```

### 2. vue3-pivot-data-table

**Status**: Recently updated (Nov 2024)  
**Repository**: https://www.npmjs.com/package/vue3-pivot-data-table  
**Installation**: `npm install vue3-pivot-data-table`

#### Features
- Inspired by Vue3-easy-data-table and Vuetify data-table
- Free and open-source
- Lightweight implementation
- Vue 3 Composition API

## Vue 2 Options (Legacy)

### 1. vue-pivot-table (Click2Buy)

**Status**: Not maintained anymore (last update: Feb 2023)  
**Repository**: https://github.com/Click2Buy/vue-pivot-table  
**Installation**: `npm install @click2buy/vue-pivot-table`

#### Features
- Drag-and-drop interface
- Customizable slots
- Large dataset support with Object.freeze
- Bootstrap dependency

#### Usage Example (Vue 2)
```javascript
import { Pivot } from '@click2buy/vue-pivot-table'
import '@click2buy/vue-pivot-table/dist/vue-pivot-table.css'

export default {
  components: { Pivot },
  data() {
    return {
      data: Object.freeze([
        { x: 0, y: 0, z: 0 },
        { x: 1, y: 1, z: 1 }
      ]),
      fields: [
        { key: 'x', getter: item => item.x, label: 'X' },
        { key: 'y', getter: item => item.y, label: 'Y' },
        { key: 'z', getter: item => item.z, label: 'Z' }
      ],
      rowFieldKeys: ['y', 'z'],
      colFieldKeys: ['x'],
      reducer: (sum, item) => sum + 1
    }
  }
}
```

### 2. vue-pivot-table-plus (LINE)

**Status**: Archived (Mar 2023)  
**Repository**: https://github.com/line/vue-pivot-table-plus  
**Installation**: `npm install vue-pivot-table-plus`

#### Features
- Enhanced version of vue-pivot-table
- CSV/TSV export functionality
- Reset functionality
- Sortable rows
- Bootstrap dependency

### 3. vue-pivottable (Original)

**Status**: Vue 2 only, Vue 3 version available separately  
**Repository**: https://github.com/Seungwoo321/vue-pivottable  
**Installation**: `npm install vue-pivottable@0.4.68`

#### Features
- Port of PivotTable.js
- Multiple renderers including Plotly
- jQuery-based functionality

## Commercial Options

### 1. Flexmonster Pivot Vue

**Status**: Commercial, actively maintained  
**Website**: https://www.flexmonster.com/pivot-grid-vue/

#### Features
- Multiple data sources: JSON, CSV, SQL, NoSQL, Elasticsearch, OLAP
- Seamless Vue 2 and Vue 3 integration
- Professional support
- Advanced reporting capabilities
- Real-time data updates

#### Pricing
- Free trial available
- Commercial license required for production use

### 2. WebDataRocks Pivot Table

**Status**: Commercial with free version  
**Website**: https://webdatarocks.com/

#### Features
- Web-based pivot table for Vue 3
- Free version with limitations
- Advanced features in paid version
- Easy integration

### 3. Syncfusion Vue Pivotview

**Status**: Commercial  
**Website**: https://ej2.syncfusion.com/vue/documentation/pivotview/

#### Features
- Enterprise-grade pivot table
- Extensive documentation
- Multiple aggregation options
- Advanced filtering and sorting

## Comparison Table

| Library | Vue Version | Maintenance | License | Key Features | Best For |
|----------|--------------|--------------|----------|--------------|-----------|
| **vue-pivottable** | Vue 3 | ✅ Active | MIT | TypeScript, Drag-drop, Charts | Modern Vue 3 projects |
| **vue3-pivot-data-table** | Vue 3 | ✅ Recent | MIT | Lightweight, Simple | Basic pivot needs |
| **vue-pivot-table** | Vue 2 | ❌ Not maintained | MIT | Slots, Customizable | Legacy Vue 2 projects |
| **vue-pivot-table-plus** | Vue 2 | ❌ Archived | Apache 2.0 | Export, Reset | Legacy projects needing export |
| **vue-pivottable (v2)** | Vue 2 | ⚠️ Vue 2 only | MIT | Plotly integration | Vue 2 with charts |
| **Flexmonster** | Vue 2/3 | ✅ Commercial | Commercial | Enterprise features | Large-scale applications |
| **WebDataRocks** | Vue 3 | ✅ Commercial | Commercial/Freemium | Web-based | Medium complexity needs |
| **Syncfusion** | Vue 3 | ✅ Commercial | Commercial | Enterprise features | Enterprise applications |

## Implementation Examples

### Basic Vue 3 Pivot Table with vue-pivottable

```vue
<template>
  <div class="pivot-container">
    <h2>Sales Data Analysis</h2>
    <VuePivottableUi
      :data="salesData"
      :rows="['product', 'category']"
      :cols="['region', 'quarter']"
      :aggregator-name="'Sum'"
      :val="'sales'"
      :renderers="renderers"
      @change="onPivotChange"
    />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { VuePivottableUi } from 'vue-pivottable'
import 'vue-pivottable/dist/vue-pivottable.css'
import PlotlyRenderer from '@vue-pivottable/plotly-renderer'

const salesData = ref([
  { product: 'Laptop', category: 'Electronics', region: 'North', quarter: 'Q1', sales: 50000 },
  { product: 'Phone', category: 'Electronics', region: 'North', quarter: 'Q1', sales: 30000 },
  { product: 'Shirt', category: 'Clothing', region: 'South', quarter: 'Q1', sales: 15000 },
  // Add more data as needed
])

const renderers = computed(() => ({
  'Table': VuePivottableUi.Table,
  'Table Barchart': VuePivottableUi.TableBarchart,
  'Grouped Column Chart': PlotlyRenderer['Grouped Column Chart'],
  'Stacked Column Chart': PlotlyRenderer['Stacked Column Chart'],
  'Line Chart': PlotlyRenderer['Line Chart']
}))

const onPivotChange = (data) => {
  console.log('Pivot configuration changed:', data)
}
</script>

<style scoped>
.pivot-container {
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
}
</style>
```

### Custom Aggregator Example

```vue
<script setup>
const customAggregators = {
  'Average': (values) => {
    return values.length > 0 ? values.reduce((a, b) => a + b) / values.length : 0
  },
  'Max': (values) => {
    return values.length > 0 ? Math.max(...values) : 0
  },
  'Min': (values) => {
    return values.length > 0 ? Math.min(...values) : 0
  }
}

// Usage in component
</script>
```

### Custom Renderer Example

```javascript
// Custom renderer for colored cells
const customRenderer = (data) => {
  const table = document.createElement('table')
  table.className = 'custom-pivot-table'
  
  // Build your custom table structure
  // Apply conditional styling based on values
  
  return table
}
```

## Recommendations

### Choose vue-pivottable (Vue 3) if:
- ✅ You're building a new Vue 3 application
- ✅ You need TypeScript support
- ✅ You want an actively maintained library
- ✅ You need interactive drag-and-drop functionality
- ✅ You want chart visualization options

### Choose vue3-pivot-data-table if:
- ✅ You need a lightweight solution
- ✅ Your requirements are simple (basic pivot functionality)
- ✅ You want minimal dependencies

### Choose Commercial Options if:
- ✅ You're building an enterprise application
- ✅ You need professional support
- ✅ You require advanced features like real-time updates
- ✅ You're working with large datasets (100K+ rows)
- ✅ You need integration with specific data sources (SQL, OLAP, etc.)

### Use Legacy Vue 2 Options if:
- ✅ You're maintaining an existing Vue 2 application
- ✅ Migration to Vue 3 is not currently possible
- ⚠️ Be aware of maintenance and security implications

## Performance Considerations

### Large Datasets
```javascript
// Use Object.freeze for large datasets to improve performance
const largeDataset = Object.freeze(yourLargeDataArray)

// Consider pagination or server-side processing for very large datasets
const paginatedData = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return largeDataset.slice(start, end)
})
```

### Memory Management
```javascript
// Clean up pivot table instances when component unmounts
onUnmounted(() => {
  // Cleanup any event listeners or large data objects
})
```

### Virtualization
For very large datasets, consider implementing virtual scrolling or server-side aggregation to maintain good performance.

## Integration with Existing Projects

### Adding to shadcn-vue Projects
Since your project uses shadcn-vue, vue-pivottable integrates well:

```vue
<template>
  <div class="space-y-4">
    <Card>
      <CardHeader>
        <CardTitle>Data Analysis</CardTitle>
      </CardHeader>
      <CardContent>
        <VuePivottableUi
          :data="analysisData"
          :rows="rows"
          :cols="cols"
          class="w-full"
        />
      </CardContent>
    </Card>
  </div>
</template>

<script setup>
import { Card, CardHeader, CardContent, CardTitle } from '@/components/ui/card'
import { VuePivottableUi } from 'vue-pivottable'
import 'vue-pivottable/dist/vue-pivottable.css'
</script>
```

## Testing Pivot Tables

### Unit Testing with Vitest
```javascript
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PivotComponent from './PivotComponent.vue'

describe('PivotComponent', () => {
  it('renders pivot table with data', async () => {
    const wrapper = mount(PivotComponent, {
      props: {
        data: [
          { category: 'A', value: 10 },
          { category: 'B', value: 20 }
        ]
      }
    })
    
    expect(wrapper.find('.pvtTable').exists()).toBe(true)
  })
})
```

## Resources

- [vue-pivottable Documentation](https://vue-pivottable.vercel.app/)
- [PivotTable.js Original](https://pivottable.js.org/)
- [Flexmonster Vue Integration](https://www.flexmonster.com/doc/vue/)
- [Vue 3 Composition API Guide](https://vuejs.org/guide/extras/composition-api-faq.html)

---

*Last updated: November 2024*
