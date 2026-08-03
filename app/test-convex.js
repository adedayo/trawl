const { ConvexClient } = require('convex/browser');
const client = new ConvexClient('http://convex:3210');
client.mutation('config:updateScopeTargets', {
  seedDomains: ['example.com', 'newdomain.com'],
  seedCidrs: [],
  seedRepos: []
}).then(() => { console.log('SUCCESS'); process.exit(0); }).catch(e => { console.error('MUTATION_ERROR:', e); process.exit(1); });
