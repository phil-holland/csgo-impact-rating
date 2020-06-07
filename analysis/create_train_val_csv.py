import click
import json
import os
import glob
from sklearn.model_selection import train_test_split
from tqdm import tqdm

@click.command()
@click.option('--train-output', '-t', type=click.File('w'), default='train.csv', show_default=True)
@click.option('--val-output', '-v', type=click.File('w'), default='val.csv', show_default=True)
@click.option('--split', '-s', type=click.FloatRange(0.0, 1.0), default=0.8, show_default=True)
@click.option('--random-seed', '-r', type=int, default=1337, show_default=True)
@click.argument('file', type=click.Path(exists=True, resolve_path=True), nargs=-1)
def create_train_val_csv(train_output, val_output, split, random_seed, file):
    """Takes in a collection of .tagged.json files and outputs two csv files, 
    one for training, one for validation. Accepts either a directory or a list
    of .tagged.json files.
    
    Example usage:

      $ create_train_val_csv.py /path/to/tagged/files/dir

      $ create_train_val_csv.py /path/to/tagged/files/dir/*.tagged.json
    """

    if os.path.isdir(file[0]):
        file = glob.glob(os.path.join(file[0], '*.tagged.json'))

    train_i, val_i = train_test_split(range(len(file)), train_size=split, random_state=random_seed)

    print('Using {} files, {} for training and {} for validation'.format(len(file), len(train_i), len(val_i)))

    # write the csv header to both files
    header = 'roundWinner,aliveCt,aliveT,meanHealthCt,meanHealthT,meanValueCT,meanValueT,roundTime,bombTime,bombDefused\n'
    train_output.write(header)
    val_output.write(header)

    train_count = 0
    val_count = 0

    # iterate over all input files
    for i, json_file in enumerate(tqdm(file, desc='Processing')):
        # load in the json data
        with open(json_file) as f:
            data = json.load(f)
        
        # loop through all ticks
        for tick in data['ticks']:
            g = tick['gameState']

            # write a single csv row
            row = (
                str(tick['roundWinner']) + ',' + str(g['aliveCT']) + ',' +
                str(g['aliveT']) + ',' + str(g['meanHealthCT']) + ',' +
                str(g['meanHealthT']) + ',' + str(g['meanValueCT']) + ',' +
                str(g['meanValueT']) + ',' + str(g['roundTime']) + ',' +
                str(g['bombTime']) + ',' + str(int(g['bombDefused'])) + '\n'
            )
            if i in train_i:
                train_output.write(row)
                train_count += 1
            elif i in val_i:
                val_output.write(row)
                val_count += 1

    print('Dataset has been split into: {} training samples, {} validation samples'.format(train_count, val_count))

if __name__ == '__main__':
    create_train_val_csv() # pylint: disable=no-value-for-parameter