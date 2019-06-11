const fs = require('fs');

const process = (key) => {
  const ws = fs.createWriteStream(`./${key}.go`);
  const items = fs.readFileSync(`./${key}.txt`, 'utf8').split(`\n`);
  ws.write(`package codephrase\n\n`);
  ws.write(`var ${key} = []string{${items.map(item => `"${item}"`).join(', ')}}\n\n`);
  ws.write(`var ${key}Map = map[string]uint64{\n`);
  items.forEach((item, idx) => ws.write(`\t"${item}": 0x${idx.toString(16)},\n`));
  ws.write('}\n');
  ws.close();
};

process('adjectives');
process('adverbs');
process('animals');
process('colors');
process('verbs');
console.log('done!');
