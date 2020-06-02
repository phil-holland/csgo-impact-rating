import click
import json
from tqdm import tqdm

@click.command()
@click.option('--output', '-o', type=click.File('w'), default='output.csv', show_default=True)
@click.argument('file', type=click.Path(exists=True, resolve_path=True), nargs=-1)
def create_csv(output, file):
    """Takes in one or more .tagged.json files and outputs a single csv
    containing all round state fields for each tick."""

    # write the csv header
    output.write('roundWinner,aliveCt,aliveT,bombPlanted,bombDefused,meanHealthCt,meanHealthT,meanValueCT,meanValueT,roundTime\n')

    for json_file in tqdm(file, desc='Processing'):
        # load in the json data
        with open(json_file) as f:
            data = json.load(f)

        last_row = ''
        
        # loop through all ticks
        for tick in data['ticks']:
            g = tick['gameState']

            # write a single csv row
            row = (
                str(tick['roundWinner']) + ',' + str(g['aliveCT']) + ',' +
                str(g['aliveT']) + ',' + str(int(g['bombPlanted'])) + ',' +
                str(int(g['bombDefused'])) + ',' + str(g['meanHealthCT']) + ',' + 
                str(g['meanHealthT']) + ',' + str(g['meanValueCT']) + ',' + 
                str(g['meanValueT']) + ',' + str(g['roundTime']) + '\n'
            )
            
            # don't write the row if it's a duplicate of the last
            if row != last_row:
                output.write(row)
            last_row = row

if __name__ == '__main__':
    create_csv() # pylint: disable=no-value-for-parameter