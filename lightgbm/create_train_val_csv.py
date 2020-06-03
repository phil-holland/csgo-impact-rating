import click
import json
from sklearn.model_selection import train_test_split

@click.command()
@click.option('--train-output', '-t', type=click.File('w'), default='train.csv', show_default=True)
@click.option('--val-output', '-v', type=click.File('w'), default='val.csv', show_default=True)
@click.option('--split', '-s', type=click.FloatRange(0.0, 1.0), default=0.8, show_default=True)
@click.option('--random-seed', '-r', type=int, default=1337, show_default=True)
@click.argument('file', type=click.File('r'))
def create_train_val_csv(train_output, val_output, split, random_seed, file):
    """Takes in a single csv file, and splits it into two output csv files, 
    one for training, one for validation."""

    rows = file.readlines()
    header = rows[0]
    rows = rows[1:]
    print(len(rows), 'rows loaded')

    train, val = train_test_split(rows, train_size=split, random_state=random_seed)
    print('Dataset has been split into train:', len(train), 'val:', len(val))

    train_output.write(header)
    val_output.write(header)
    
    for row in train:
        train_output.write(row)
    for row in val:
        val_output.write(row)

if __name__ == '__main__':
    create_train_val_csv() # pylint: disable=no-value-for-parameter